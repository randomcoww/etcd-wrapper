package etcdutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/util/constants"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func newClient(clientURLs []string, tc *tls.Config) (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints:   clientURLs,
		DialTimeout: constants.DefaultDialTimeout,
		TLS:         tc,
	})
}

func Status(clientURLs []string, tc *tls.Config) (*clientv3.StatusResponse, error) {
	etcdcli, err := newClient(clientURLs, tc)
	if err != nil {
		return nil, err
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	resp, err := etcdcli.Status(ctx, clientURLs[0])
	cancel()
	return resp, err
}

func AddMember(clientURLs, peerURLs []string, tc *tls.Config) (*clientv3.MemberAddResponse, error) {
	etcdcli, err := newClient(clientURLs, tc)
	if err != nil {
		return nil, err
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	resp, err := etcdcli.Cluster.MemberAdd(ctx, peerURLs)
	cancel()
	return resp, err
}

// https://github.com/coreos/etcd-operator/blob/master/pkg/util/etcdutil/etcdutil.go
func ListMembers(clientURLs []string, tc *tls.Config) (*clientv3.MemberListResponse, error) {
	cfg := clientv3.Config{
		Endpoints:   clientURLs,
		DialTimeout: constants.DefaultDialTimeout,
		TLS:         tc,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("list members failed: creating etcd client failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	resp, err := etcdcli.MemberList(ctx)
	cancel()
	etcdcli.Close()
	return resp, err
}

// https://github.com/coreos/etcd-operator/blob/master/pkg/util/etcdutil/etcdutil.go
func RemoveMember(clientURLs []string, tc *tls.Config, id uint64) error {
	cfg := clientv3.Config{
		Endpoints:   clientURLs,
		DialTimeout: constants.DefaultDialTimeout,
		TLS:         tc,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return err
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	_, err = etcdcli.Cluster.MemberRemove(ctx, id)
	cancel()
	return err
}

// https://github.com/etcd-io/etcd/blob/ff455e3567d6465717d3ced6dc889d4eec9ed890/etcdctl/ctlv3/command/ep_command.go#L121
func HealthCheck(clientURLs []string, tc *tls.Config) error {
	etcdcli, err := newClient(clientURLs, tc)
	if err != nil {
		return err
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	_, err = etcdcli.Get(ctx, "health")
	cancel()

	switch err {
	case nil, rpctypes.ErrPermissionDenied:
		return nil
	default:
		return err
	}
}
