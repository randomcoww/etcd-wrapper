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

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	resp, err := etcdcli.Status(ctx, clientURLs[0])
	cancel()
	etcdcli.Close()
	return resp, err
}

func AddMember(clientURLs, peerURLs []string, tc *tls.Config) error {
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
	_, err = etcdcli.Cluster.MemberAdd(ctx, peerURLs)
	cancel()
	return err
}