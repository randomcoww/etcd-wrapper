package etcdclient

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"io"
	"net"
	"net/http"
	"time"
)

type Client struct {
	*clientv3.Client
}

type StatusResponse struct {
	*etcdserverpb.StatusResponse
}

type MemberListResponse struct {
	*etcdserverpb.MemberListResponse
}

type Members interface {
	GetHeader() *etcdserverpb.ResponseHeader
	GetMembers() []*etcdserverpb.Member
}

type Member interface {
	GetID() uint64
	GetName() string
	GetClientURLs() []string
	GetPeerURLs() []string
}

type Status interface {
	GetHeader() *etcdserverpb.ResponseHeader
	GetLeader() uint64
}

type Header interface {
	GetClusterId() uint64
	GetMemberId() uint64
	GetRevision() int64
}

type EtcdClient interface {
	Status(context.Context, string) (Status, error)
	MemberList(context.Context) (Members, error)
	MemberAdd(context.Context, []string) (Members, error)
	MemberRemove(context.Context, uint64) (Members, error)
	GetHealth(context.Context) error
	Defragment(context.Context, string) error
	Snapshot(context.Context) (io.Reader, error)
}

const (
	dialTimeout        time.Duration = 2 * time.Second
	backoffWaitBetween time.Duration = 2 * time.Second
)

func NewClientFromPeers(ctx context.Context, config *c.Config) (EtcdClient, error) {
	tick := time.NewTicker(backoffWaitBetween)
	defer tick.Stop()

	for {
		pcluster, err := etcdserver.GetClusterFromRemotePeers(config.Logger, config.ClusterPeerURLs, &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: 30 * time.Second, // value taken from http.DefaultTransport
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second, // value taken from http.DefaultTransport
			TLSClientConfig:     config.PeerTLSConfig,
		})
		if err == nil {
			client, err := NewClient(ctx, config, pcluster.ClientURLs())
			if err == nil {
				return client, nil
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-tick.C:
			continue
		}
	}
}

func NewClient(ctx context.Context, config *c.Config, endpoints []string) (EtcdClient, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		DialTimeout:          dialTimeout,
		TLS:                  config.ClientTLSConfig,
		DialKeepAliveTime:    2 * time.Second,
		DialKeepAliveTimeout: 2 * time.Second,
		BackoffWaitBetween:   backoffWaitBetween,
		Context:              ctx,
		Logger:               config.Logger,
	})
	if err != nil {
		return nil, err
	}
	return &Client{
		client,
	}, nil
}

func (client *Client) MemberList(ctx context.Context) (Members, error) {
	resp, err := client.Cluster.MemberList(ctx)
	if err != nil {
		return nil, err
	}
	return (*etcdserverpb.MemberListResponse)(resp), nil
}

func (client *Client) Status(ctx context.Context, endpoint string) (Status, error) {
	resp, err := client.Maintenance.Status(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	return (*etcdserverpb.StatusResponse)(resp), nil
}

func (client *Client) MemberAdd(ctx context.Context, peerURLs []string) (Members, error) {
	resp, err := client.Cluster.MemberAdd(ctx, peerURLs)
	if err != nil {
		return nil, err
	}
	return (*etcdserverpb.MemberAddResponse)(resp), nil
}

func (client *Client) MemberRemove(ctx context.Context, id uint64) (Members, error) {
	resp, err := client.Cluster.MemberRemove(ctx, id)
	if err != nil {
		return nil, err
	}
	return (*etcdserverpb.MemberRemoveResponse)(resp), nil
}

func (client *Client) GetHealth(ctx context.Context) error {
	_, err := client.Get(ctx, "health")
	return err
}

func (client *Client) Defragment(ctx context.Context, endpoint string) error {
	_, err := client.Maintenance.Defragment(ctx, endpoint)
	return err
}

func (client *Client) Snapshot(ctx context.Context) (io.Reader, error) {
	rc, err := client.Maintenance.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return rc, nil
}
