package etcdutil

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/coreos/etcd-operator/pkg/util/constants"
	"go.etcd.io/etcd/clientv3"
)

func Status(clientURLs []string, tc *tls.Config) (*clientv3.StatusResponse, error) {
	cfg := clientv3.Config{
		Endpoints:   clientURLs,
		DialTimeout: constants.DefaultDialTimeout,
		TLS:         tc,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("get cluster status failed: %v", err)
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	resp, err := etcdcli.Status(ctx, clientURLs[0])
	cancel()
	return resp, err
}

func AddMember(clientURLs, peerURLs []string, tc *tls.Config) (*clientv3.MemberAddResponse, error) {
	cfg := clientv3.Config{
		Endpoints:   clientURLs,
		DialTimeout: constants.DefaultDialTimeout,
		TLS:         tc,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	resp, err := etcdcli.Cluster.MemberAdd(ctx, peerURLs)
	cancel()
	return resp, err
}
