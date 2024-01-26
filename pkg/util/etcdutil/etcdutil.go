package etcdutil

import (
	"context"
	"crypto/tls"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	"github.com/randomcoww/etcd-wrapper/pkg/util/s3util"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"sync"
	"time"
)

const (
	defaultRequestTimeout time.Duration = 5 * time.Second
	defaultDialTimeout    time.Duration = 5 * time.Second
	snapshotBackupTimeout time.Duration = 1 * time.Minute
	defragmentTimeout     time.Duration = 1 * time.Minute
)

type StatusResp struct {
	Endpoint string
	Status   *clientv3.StatusResponse
	Err      error
}

func new(ctx context.Context, endpoints []string, tlsConfig *tls.Config) (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: defaultDialTimeout,
		TLS:         tlsConfig,
		Context:     ctx,
	})
}

func Status(endpoints []string, tlsConfig *tls.Config) (chan *StatusResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	client, err := new(ctx, endpoints, tlsConfig)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var wg sync.WaitGroup
	respCh := make(chan *StatusResp, len(endpoints))
	for _, endpoint := range endpoints {
		wg.Add(1)
		go func(endpoint string) {
			status, err := client.Status(ctx, endpoint)
			respCh <- &StatusResp{
				Endpoint: endpoint,
				Status:   status,
				Err:      err,
			}
			wg.Done()
		}(endpoint)
	}
	wg.Wait()
	return respCh, nil
}

func AddMember(endpoints, peerURLs []string, tlsConfig *tls.Config) (*clientv3.MemberAddResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	client, err := new(ctx, endpoints, tlsConfig)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	resp, err := client.Cluster.MemberAdd(ctx, peerURLs)
	return resp, err
}

func RemoveMember(endpoints []string, tlsConfig *tls.Config, id uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	client, err := new(ctx, endpoints, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Cluster.MemberRemove(ctx, id)
	return err
}

func ListMembers(endpoints []string, tlsConfig *tls.Config) (*clientv3.MemberListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	client, err := new(ctx, endpoints, tlsConfig)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	resp, err := client.MemberList(ctx)
	return resp, err
}

func HealthCheck(endpoints []string, tlsConfig *tls.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	client, err := new(ctx, endpoints, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Get(ctx, "health")
	switch err {
	case nil, rpctypes.ErrPermissionDenied:
		return nil
	default:
		return err
	}
}

func Defragment(endpoint string, tlsConfig *tls.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), defragmentTimeout)
	defer cancel()

	client, err := new(ctx, []string{endpoint}, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Defragment(ctx, endpoint)
	return err
}

func BackupSnapshot(endpoints []string, s3Resource string, s3 *s3util.Client, tlsConfig *tls.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), snapshotBackupTimeout)
	defer cancel()

	client, err := new(ctx, endpoints, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	readCloser, err := client.Snapshot(ctx)
	if err != nil {
		return err
	}
	defer readCloser.Close()

	_, err = s3.Write(ctx, s3Resource, readCloser)
	return err
}

func RestoreSnapshot(restoreFile string, s3resource string, s3 *s3util.Client) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), snapshotBackupTimeout)
	defer cancel()

	readCloser, err := s3.Open(ctx, s3resource)
	if err != nil {
		return false, err
	}
	if readCloser == nil {
		return false, nil
	}
	defer readCloser.Close()

	err = util.WriteFile(readCloser, restoreFile)
	if err != nil {
		return false, err
	}
	return true, nil
}
