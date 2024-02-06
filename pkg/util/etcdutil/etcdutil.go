package etcdutil

import (
	"context"
	"crypto/tls"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"io"
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

func CreateSnapshot(endpoints []string, tlsConfig *tls.Config, handler func(context.Context, io.Reader) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), snapshotBackupTimeout)
	defer cancel()

	client, err := new(ctx, endpoints, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()
	rc, err := client.Snapshot(ctx)
	if err != nil {
		return err
	}
	defer rc.Close()
	return handler(ctx, rc)
}
