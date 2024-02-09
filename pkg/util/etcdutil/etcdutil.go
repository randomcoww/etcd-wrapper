package etcdutil

import (
	"context"
	"crypto/tls"
	"fmt"
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
	Endpoint  string
	ClusterID *uint64
	MemberID  *uint64
	LeaderID  *uint64
	Revision  *int64
}

type MemberResp struct {
	ID       *uint64
	PeerURLs []string
}

type StatusCheck interface {
	Status(endpoints []string, handler func(*StatusResp, error), tlsConfig *tls.Config) error
	ListMembers(endpoints []string, tlsConfig *tls.Config) ([]*MemberResp, error)
	HealthCheck(endpoints []string, tlsConfig *tls.Config) error
}

func new(ctx context.Context, endpoints []string, tlsConfig *tls.Config) (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: defaultDialTimeout,
		TLS:         tlsConfig,
		Context:     ctx,
	})
}

func Status(endpoints []string, handler func(*StatusResp, error), tlsConfig *tls.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	client, err := new(ctx, endpoints, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	var wg sync.WaitGroup
	defer wg.Wait()

	for _, endpoint := range endpoints {
		wg.Add(1)
		go func(ctx context.Context, endpoint string) {
			defer wg.Done()

			status := &StatusResp{
				Endpoint: endpoint,
			}
			resp, err := client.Status(ctx, endpoint)
			if err != nil {
				handler(status, err)
				return
			}
			clusterID := resp.Header.ClusterId
			memberID := resp.Header.MemberId
			leaderID := resp.Leader
			revision := resp.Header.Revision

			if clusterID == 0 {
				handler(status, fmt.Errorf("Cluster ID is 0"))
				return
			}
			status.ClusterID = &clusterID
			if memberID != 0 {
				status.MemberID = &memberID
			}
			if leaderID != 0 {
				status.LeaderID = &leaderID
			}
			status.Revision = &revision

			handler(status, nil)
		}(ctx, endpoint)
	}
	return nil
}

func AddMember(endpoints, peerURLs []string, tlsConfig *tls.Config) (*MemberResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	client, err := new(ctx, endpoints, tlsConfig)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	resp, err := client.Cluster.MemberAdd(ctx, peerURLs)
	if err != nil {
		return nil, err
	}
	id := resp.Member.ID
	if id == 0 {
		return nil, fmt.Errorf("Member ID is 0")
	}
	return &MemberResp{
		ID:       &id,
		PeerURLs: resp.Member.PeerURLs,
	}, nil
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

func ListMembers(endpoints []string, tlsConfig *tls.Config) ([]*MemberResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	client, err := new(ctx, endpoints, tlsConfig)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var members []*MemberResp
	resp, err := client.MemberList(ctx)
	if err != nil {
		return members, err
	}
	for _, member := range resp.Members {
		id := member.ID
		if id != 0 {
			members = append(members, &MemberResp{
				ID:       &id,
				PeerURLs: member.PeerURLs,
			})
		}
	}
	return members, nil
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
