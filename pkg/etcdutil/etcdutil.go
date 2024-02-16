package etcdutil

import (
	"context"
	"crypto/tls"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
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

type Client struct {
	*clientv3.Client
}

type List interface {
	GetHeader() *etcdserverpb.ResponseHeader
	GetMembers() []*etcdserverpb.Member
}

type Status interface {
	GetHeader() *etcdserverpb.ResponseHeader
	GetIsLearner() bool
	GetLeader() uint64
	GetRaftIndex() uint64
}

type Header interface {
	GetClusterId() uint64
	GetMemberId() uint64
	GetRevision() int64
}

type Member interface {
	GetID() uint64
	GetName() string
	GetClientURLs() []string
	GetPeerURLs() []string
	GetIsLearner() bool
}

type Util interface {
	Close() error
	Sync(context.Context) error
	Endpoints() []string
	SyncEndpoints() error
	Status(handler func(Status, error))
	ListMembers() (List, error)
	AddMember(peerURLs []string) (List, Member, error)
	RemoveMember(id uint64) (List, error)
	PromoteMember(id uint64) (List, error)
	HealthCheck() error
	Defragment(endpoint string) error
	CreateSnapshot(handler func(context.Context, io.Reader) error) error
}

func New(endpoints []string, tlsConfig *tls.Config) (Util, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: defaultDialTimeout,
		TLS:         tlsConfig,
		Context:     context.Background(),
	})
	if err != nil {
		return nil, err
	}
	return &Client{
		Client: client,
	}, nil
}

func (client *Client) SyncEndpoints() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	return client.Sync(ctx)
}

func (client *Client) Status(handler func(Status, error)) {
	var wg sync.WaitGroup
	defer wg.Wait()

	for _, endpoint := range client.Endpoints() {
		wg.Add(1)
		go func(endpoint string) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
			defer cancel()

			resp, err := client.Maintenance.Status(ctx, endpoint)
			handler((*etcdserverpb.StatusResponse)(resp), err)
		}(endpoint)
	}
}

func (client *Client) ListMembers() (List, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	resp, err := client.Cluster.MemberList(ctx)
	return (*etcdserverpb.MemberListResponse)(resp), err
}

func (client *Client) AddMember(peerURLs []string) (List, Member, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()

	resp, err := client.Cluster.MemberAddAsLearner(ctx, peerURLs)
	if err != nil {
		return nil, nil, err
	}
	addResp := (*etcdserverpb.MemberAddResponse)(resp)
	return addResp, addResp.GetMember(), nil
}

func (client *Client) RemoveMember(id uint64) (List, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	resp, err := client.Cluster.MemberRemove(ctx, id)
	return (*etcdserverpb.MemberRemoveResponse)(resp), err
}

func (client *Client) PromoteMember(id uint64) (List, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	resp, err := client.Cluster.MemberPromote(ctx, id)
	return (*etcdserverpb.MemberPromoteResponse)(resp), err
}

func (client *Client) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultRequestTimeout)
	defer cancel()
	_, err := client.Get(ctx, "health")
	switch err {
	case nil, rpctypes.ErrPermissionDenied:
		return nil
	default:
		return err
	}
}

func (client *Client) Defragment(endpoint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defragmentTimeout)
	defer cancel()
	_, err := client.Maintenance.Defragment(ctx, endpoint)
	return err
}

func (client *Client) CreateSnapshot(handler func(context.Context, io.Reader) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), snapshotBackupTimeout)
	defer cancel()
	rc, err := client.Maintenance.Snapshot(ctx)
	if err != nil {
		return err
	}
	defer rc.Close()
	return handler(ctx, rc)
}
