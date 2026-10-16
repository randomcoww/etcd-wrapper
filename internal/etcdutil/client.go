package etcdutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/randomcoww/etcd-wrapper/internal/util"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.uber.org/zap"
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
	MemberUpdate(context.Context, uint64, []string) (Members, error)
	MemberRemove(context.Context, uint64) (Members, error)
	GetRevision(context.Context) (int64, error)
	CheckQuorum(context.Context) error
	Defragment(context.Context, string) error
	Snapshot(context.Context) (io.ReadCloser, error)
	Close() error
	C() *clientv3.Client
}

const (
	dialTimeout        time.Duration = 2 * time.Second
	backoffWaitBetween time.Duration = 2 * time.Second
)

func NewClientFromPeers(ctx context.Context, logger *zap.Logger, peerURLs []string, peerTLSConfig, clienTLSConfig *tls.Config) (EtcdClient, error) {
	for {
		pcluster, err := etcdserver.GetClusterFromRemotePeers(logger, peerURLs, &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: 30 * time.Second, // value taken from http.DefaultTransport
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second, // value taken from http.DefaultTransport
			TLSClientConfig:     peerTLSConfig,
		})
		if err == nil {
			client, err := NewClient(ctx, logger, pcluster.ClientURLs(), clienTLSConfig)
			if err == nil {
				logger.Info("found cluster", zap.String("URLs", fmt.Sprintf("%v", pcluster.ClientURLs())))
				return client, nil
			}
		}

		timer := time.NewTimer(backoffWaitBetween)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %w", ctx.Err(), err)

		case <-timer.C:
			continue
		}
	}
}

func NewClient(ctx context.Context, logger *zap.Logger, endpoints []string, clienTLSConfig *tls.Config) (EtcdClient, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:            endpoints,
		DialTimeout:          dialTimeout,
		TLS:                  clienTLSConfig,
		DialKeepAliveTime:    2 * time.Second,
		DialKeepAliveTimeout: 2 * time.Second,
		BackoffWaitBetween:   backoffWaitBetween,
		Context:              ctx,
		Logger:               logger,
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
		return nil, fmt.Errorf("get member list: %w", err)
	}
	return (*etcdserverpb.MemberListResponse)(resp), nil
}

func (client *Client) Status(ctx context.Context, endpoint string) (Status, error) {
	resp, err := client.Maintenance.Status(ctx, endpoint)
	if err != nil {
		return nil, fmt.Errorf("get node status: %w", err)
	}
	return (*etcdserverpb.StatusResponse)(resp), nil
}

func (client *Client) MemberAdd(ctx context.Context, peerURLs []string) (Members, error) {
	for {
		resp, err := client.Cluster.MemberAdd(ctx, peerURLs)
		switch {
		case err == nil:
			return (*etcdserverpb.MemberAddResponse)(resp), nil
		default:
		}

		timer := time.NewTimer(backoffWaitBetween)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("add member %w: %w", ctx.Err(), err)
		case <-timer.C:
			continue
		}
	}
}

func (client *Client) MemberRemove(ctx context.Context, id uint64) (Members, error) {
	for {
		resp, err := client.Cluster.MemberRemove(ctx, id)
		switch {
		case err == nil:
			return (*etcdserverpb.MemberRemoveResponse)(resp), nil
		default:
		}

		timer := time.NewTimer(backoffWaitBetween)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("remove member %w: %w", ctx.Err(), err)
		case <-timer.C:
			continue
		}
	}
}

func (client *Client) MemberUpdate(ctx context.Context, id uint64, peerURLs []string) (Members, error) {
	for {
		resp, err := client.Cluster.MemberUpdate(ctx, id, peerURLs)
		switch {
		case err == nil:
			return (*etcdserverpb.MemberPromoteResponse)(resp), nil
		default:
		}

		timer := time.NewTimer(backoffWaitBetween)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("update member %w: %w", ctx.Err(), err)
		case <-timer.C:
			continue
		}
	}
}

func (client *Client) GetRevision(ctx context.Context) (int64, error) {
	resp, err := client.Get(ctx, "health-check-dummy", clientv3.WithCountOnly())
	if err != nil {
		return 0, fmt.Errorf("get cluster revision: %w", err)
	}
	return resp.Header.Revision, nil
}

func (client *Client) CheckQuorum(ctx context.Context) error {
	if _, err := client.GetRevision(ctx); err != nil {
		return fmt.Errorf("check quorum: %w", err)
	}
	return nil
}

func (client *Client) Defragment(ctx context.Context, endpoint string) error {
	if _, err := client.Maintenance.Defragment(ctx, endpoint); err != nil {
		return fmt.Errorf("defragment node: %w", err)
	}
	return nil
}

func (client *Client) Snapshot(ctx context.Context) (io.ReadCloser, error) {
	rc, err := client.Maintenance.Snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot data: %w", err)
	}
	return rc, nil
}

func (client *Client) C() *clientv3.Client {
	return client.Client
}

func FindLocalMemberInResp(listResp Members, name string, peerURLs []string) Member {
	for _, member := range listResp.GetMembers() {
		if member.GetName() == name {
			return member
		}
		if util.HasMatchingElement(member.GetPeerURLs(), peerURLs) {
			return member
		}
	}
	return nil
}
