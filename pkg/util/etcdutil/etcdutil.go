package etcdutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	"github.com/randomcoww/etcd-wrapper/pkg/util/s3util"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"os"
	"sync"
	"time"
)

const (
	defaultRequestTimeout time.Duration = 4 * time.Second
	defaultDialTimeout    time.Duration = 4 * time.Second
	snapshotBackupTimeout time.Duration = 2 * time.Minute
)

type StatusResp struct {
	Endpoint string
	Status   *clientv3.StatusResponse
	Err      error
}

func newClient(ctx context.Context, endpoints []string, tlsConfig *tls.Config) (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: defaultDialTimeout,
		TLS:         tlsConfig,
		Context:     ctx,
	})
}

func Status(endpoints []string, tlsConfig *tls.Config) (chan *StatusResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	client, err := newClient(ctx, endpoints, tlsConfig)
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
	cancel()
	return respCh, nil
}

func AddMember(endpoints, peerURLs []string, tlsConfig *tls.Config) (*clientv3.MemberAddResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	client, err := newClient(ctx, endpoints, tlsConfig)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	resp, err := client.Cluster.MemberAdd(ctx, peerURLs)
	cancel()
	return resp, err
}

func RemoveMember(endpoints []string, tlsConfig *tls.Config, id uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	client, err := newClient(ctx, endpoints, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Cluster.MemberRemove(ctx, id)
	cancel()
	return err
}

func ListMembers(endpoints []string, tlsConfig *tls.Config) (*clientv3.MemberListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	client, err := newClient(ctx, endpoints, tlsConfig)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	resp, err := client.MemberList(ctx)
	cancel()
	return resp, err
}

func HealthCheck(endpoints []string, tlsConfig *tls.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	client, err := newClient(ctx, endpoints, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Get(ctx, "health")
	cancel()

	switch err {
	case nil, rpctypes.ErrPermissionDenied:
		return nil
	default:
		return err
	}
}

func BackupSnapshot(endpoints []string, s3Resource string, writer s3util.Writer, tlsConfig *tls.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), snapshotBackupTimeout)
	client, err := newClient(ctx, endpoints, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	readCloser, err := client.Snapshot(ctx)
	if err != nil {
		return err
	}
	defer readCloser.Close()

	_, err = writer.Write(ctx, s3Resource, readCloser)
	cancel()
	return err
}

func RestoreSnapshot(restoreFile string, s3resource string, reader s3util.Reader) error {
	readCloser, err := reader.Open(s3resource)
	if err != nil {
		return err
	}
	defer readCloser.Close()

	err = util.WriteFile(readCloser, restoreFile)
	if err != nil {
		return err
	}

	info, err := os.Stat(restoreFile)
	if err != nil {
		return err
	}

	if info.Size() == 0 {
		return fmt.Errorf("Snapshot file size is 0")
	}
	return nil
}
