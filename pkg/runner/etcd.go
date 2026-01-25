package runner

import (
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.uber.org/zap"
	"os"
	"time"
)

func RunEtcd(ctx context.Context, config *c.Config, p etcdprocess.EtcdProcess, s3 s3client.Client) error {
	defer config.Logger.Sync()

	// always clean out data
	// data can be recreated from cluster
	// data restore is needed on full cluster restart
	if d, ok := config.Env["ETCD_DATA_DIR"]; ok && d != "" {
		if err := removeDir(d); err != nil {
			config.Logger.Error("remove data dir", zap.Error(err))
			return err
		}
	}
	if d, ok := config.Env["ETCD_WAL_DIR"]; ok && d != "" {
		if err := removeDir(d); err != nil {
			config.Logger.Error("remove wal dir", zap.Error(err))
			return err
		}
	}

	// wait for existing cluster (and quorum)
	clusterCtx, clusterCancel := context.WithTimeout(ctx, time.Duration(config.ClusterTimeout))
	defer clusterCancel()

	client, err := etcdclient.NewClientFromPeers(clusterCtx, config)
	if err != nil {
		// no members found
		config.Logger.Info("no members found")

		ok, err := backup.RestoreSnapshot(ctx, config, s3, 1000000000)
		if err != nil {
			return err
		}
		if !ok {
			config.Logger.Info("starting member new fresh")
			return p.StartEtcdNew(ctx, config)
		}

		config.Logger.Info("starting member existing with backup data")
		return p.StartEtcdExisting(ctx, config)
	}
	defer client.Close()

	config.Logger.Info("existing members found")
	// found members - check if quorum is established
	if err := client.GetQuorum(clusterCtx); err != nil {
		config.Logger.Info("no quorum found")

		config.Logger.Info("starting member existing")
		return p.StartEtcdExisting(ctx, config)
	}

	config.Logger.Info("quorum found")
	// cluster with quorum found - this is the most common scenario
	clientCtx, clientCancel := context.WithTimeout(ctx, time.Duration(config.ReplaceTimeout))
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
	return p.StartEtcdExisting(ctx, config)
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
