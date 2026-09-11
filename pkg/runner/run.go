package runner

import (
	"context"
	"go.uber.org/zap"
	"os"
	"time"

	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
)

type etcdProcess interface {
	StartNew(*c.Config) error
	StartExisting(*c.Config) error
	Stop() error
	Wait() error
}

func RunEtcd(ctx context.Context, config *c.Config, etcdRunner etcdProcess) error {
	// wait for existing cluster (and quorum)
	clusterCtx, clusterCancel := context.WithTimeout(ctx, time.Duration(config.InitialClusterTimeout))
	defer clusterCancel()

	// local data revision, err if broken, revision > 0 if data exists
	revision, err := etcdclient.GetDataRevision(config)
	if err != nil {
		config.Logger.Error("get local data revision", zap.Error(err))

		if err = clearExistingData(config); err != nil {
			config.Logger.Error("delete local data", zap.Error(err))
			return err
		}
	}

	client, err := etcdclient.NewClientFromPeers(clusterCtx, config)
	if err != nil {
		// no members found
		config.Logger.Info("no members found")

		if revision == 0 {
			// no local data - start fresh
			config.Logger.Info("starting member new fresh")
			return etcdRunner.StartNew(config)
		}

		config.Logger.Info("starting member existing with backup data")
		return etcdRunner.StartExisting(config)
	}
	defer client.Close()

	// at least one member found
	config.Logger.Info("existing members found")

	// found members - check if quorum is established
	remoteRevision, err := client.GetRevision(clusterCtx)
	if err != nil {
		config.Logger.Info("no quorum found")
		config.Logger.Info("starting member existing")
		return etcdRunner.StartExisting(config)
	}

	config.Logger.Info("quorum found")
	if revision >= remoteRevision {
		return etcdRunner.StartExisting(config)
	}

	// cluster with quorum found - this is the most common scenario
	clientCtx, clientCancel := context.WithTimeout(ctx, time.Duration(config.ClientTimeout*2))
	defer clientCancel()

	listResp, err := client.MemberList(clientCtx)
	if err != nil {
		config.Logger.Error("list member failed", zap.Error(err))
		return err
	}
	localMember := findLocalMember(listResp, config)

	// replace my node to join cluster
	// if my node already exists, it needs to be replaced
	if localMember != nil && len(listResp.GetMembers()) >= len(config.ClusterPeerURLs) {
		listResp, err = client.MemberRemove(clientCtx, localMember.GetID())
		if err != nil {
			config.Logger.Error("remove member failed", zap.Error(err))
			return err
		}
		localMember = findLocalMember(listResp, config)
		config.Logger.Info("removed local member")
	}

	if localMember == nil && len(listResp.GetMembers()) < len(config.ClusterPeerURLs) {
		_, err = client.MemberAdd(clientCtx, config.InitialAdvertisePeerURLs)
		if err != nil {
			config.Logger.Error("add member failed", zap.Error(err))
			return err
		}
		config.Logger.Info("added local member")
	}

	config.Logger.Info("starting member existing")
	if err = clearExistingData(config); err != nil {
		return err
	}
	return etcdRunner.StartExisting(config)
}

func clearExistingData(config *c.Config) error {
	if d, ok := config.Env["ETCD_DATA_DIR"]; ok && d != "" {
		if err := removeDir(d); err != nil {
			config.Logger.Error("remove data dir", zap.Error(err))
			return err
		}
	}
	config.Logger.Info("cleaned out existing data")
	return nil
}

func findLocalMember(listResp etcdclient.Members, config *c.Config) *etcdserverpb.Member {
	for _, member := range listResp.GetMembers() {
		if member.GetName() == config.Env["ETCD_NAME"] {
			return member
		}
		if util.HasMatchingElement(member.GetPeerURLs(), config.InitialAdvertisePeerURLs) {
			return member
		}
	}
	return nil
}

func removeDir(path string) error {
	_, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	}
	return os.RemoveAll(path)
}
