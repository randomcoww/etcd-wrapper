package etcdutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/util/constants"
	"go.etcd.io/etcd/clientv3"
)

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
