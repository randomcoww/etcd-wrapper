package etcdutil

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/util/constants"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func newClient(ctx context.Context, endpoints []string, tlsConfig *tls.Config) (*clientv3.Client, error) {
	return clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: constants.DefaultDialTimeout,
		TLS:         tlsConfig,
		Context: ctx,
	})
}

func AddMember(endpoints, peerURLs []string, tlsConfig *tls.Config) (*clientv3.MemberAddResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	client, err := newClient(ctx, endpoints, tlsConfig)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	resp, err := client.Cluster.MemberAdd(ctx, peerURLs)
	cancel()
	return resp, err
}

func ListMembers(endpoints []string, tlsConfig *tls.Config) (*clientv3.MemberListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
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

func RemoveMember(endpoints []string, tlsConfig *tls.Config, id uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	client, err := newClient(ctx, endpoints, tlsConfig)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.Cluster.MemberRemove(ctx, id)
	cancel()
	return err
}

func HealthCheck(endpoints []string, tlsConfig *tls.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
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

// func GetIsMaxRevision(endpoints []string, tlsConfig *tls.Config) error {
// 	for _, endpoint := range endpoints {
// 		ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
// 		client, err := newClient(ctx, []string{endpoint}, tlsConfig)

// 	}
// }

// func getRevision(endpoints []string, tlsConfig *tls.Config) (int64, *clientv3.Client, error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
// 	client, err := newClient(ctx, endpoints, tlsConfig)
// 	if err != nil {
// 		return 0, err
// 	}
// 	defer client.Close()

// 	resp, err := client.Get(ctx, "/", clientv3.WithSerializable())
// 	cancel()

// 	switch err {
// 	case nil
// 		return resp.Header.Revision, client, nil
// 	default:
// 		return 0, nil, err
// 	}
// }

// func GetMaxRevisionClient(endpoints []string,  tlsConfig *tls.Config) (*clientv3.Client, error) {
// 	var maxRev int64
// 	var maxRevClient *clientv3.Client

// 	for _, endpoint := range endpoints {
// 		rev, client, err := getRevision([]string{endpoint}, tlsConfig)
// 		if err != nil {
// 			return nil, err
// 		}

// 		if rev > maxRev {
// 			maxRev = rev
// 			maxRevClient = client
// 		}
// 	}
// }

// func GetClientWithMaxRev(ctx context.Context, endpoints []string, tlsConfig *tls.Config) (*clientv3.Client, int64, error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	
// 	mapEps := make(map[string]*clientv3.Client)
// 	var maxClient *clientv3.Client
// 	maxRev := int64(0)
// 	errors := make([]string, 0)
// 	for _, endpoint := range endpoints {
// 		cfg := clientv3.Config{
// 			Endpoints:   []string{endpoint},
// 			DialTimeout: constants.DefaultDialTimeout,
// 			TLS:         tc,
// 			Context: ctx,
// 		}
// 		etcdcli, err := clientv3.New(cfg)
// 		if err != nil {
// 			errors = append(errors, fmt.Sprintf("failed to create etcd client for endpoint (%v): %v", endpoint, err))
// 			continue
// 		}
// 		mapEps[endpoint] = etcdcli

// 		resp, err := etcdcli.Get(ctx, "/", clientv3.WithSerializable())
// 		if err != nil {
// 			errors = append(errors, fmt.Sprintf("failed to get revision from endpoint (%s)", endpoint))
// 			continue
// 		}

// 		logrus.Infof("getMaxRev: endpoint %s revision (%d)", endpoint, resp.Header.Revision)
// 		if resp.Header.Revision > maxRev {
// 			maxRev = resp.Header.Revision
// 			maxClient = etcdcli
// 		}
// 	}

// 	// close all open clients that are not maxClient.
// 	for _, cli := range mapEps {
// 		if cli == maxClient {
// 			continue
// 		}
// 		cli.Close()
// 	}

// 	if maxClient == nil {
// 		return nil, 0, fmt.Errorf("could not create an etcd client for the max revision purpose from given endpoints (%v)", endpoints)
// 	}

// 	var err error
// 	if len(errors) > 0 {
// 		errorStr := ""
// 		for _, errStr := range errors {
// 			errorStr += errStr + "\n"
// 		}
// 		err = fmt.Errorf(errorStr)
// 	}

// 	return maxClient, maxRev, err
// }
