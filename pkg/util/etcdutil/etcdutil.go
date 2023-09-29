package etcdutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"time"
)

const (
	defaultRequestTimeout time.Duration = 2 * time.Second
	defaultDialTimeout    time.Duration = 2 * time.Second
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
	ctx, _ := context.WithTimeout(context.Background(), defaultRequestTimeout)
	client, err := newClient(ctx, endpoints, tlsConfig)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	respCh := make(chan *StatusResp, len(endpoints))
	for _, endpoint := range endpoints {
		go func(endpoint string) {
			status, err := client.Status(ctx, endpoint)
			respCh <- &StatusResp{
				Endpoint: endpoint,
				Status:   status,
				Err:      err,
			}
		}(endpoint)
	}
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
		return nil, fmt.Errorf("list members failed: creating etcd client failed: %v", err)
	}
	defer client.Close()

	resp, err := client.MemberList(ctx)
	cancel()
	client.Close()
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

// func Snapshot(endpoints []string, tlsConfig *tls.Config) (*io.ReadCloser, func(){}, error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
// 	client, err := newClient(ctx, endpoints, tlsConfig)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	defer client.Close()

// 	readCloser, err := client.Snapshot(ctx)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	// defer readCloser.Close()
// 	// cancel()

// 	switch err {
// 	case nil, rpctypes.ErrPermissionDenied:
// 		return nil
// 	default:
// 		return err
// 	}
// }
