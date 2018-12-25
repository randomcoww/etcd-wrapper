package etcdutil

import (
	"context"
	"crypto/tls"

	"github.com/coreos/etcd-operator/pkg/util/constants"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
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
