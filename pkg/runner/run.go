package runner

import (
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.uber.org/zap"
	"os"
	"time"
)

type etcdProcess interface {
	StartNew(*c.Config) error
	StartExisting(*c.Config) error
	Stop() error
	Wait() error
}

const (
	restoreVersionBump uint64 = 1000000000
)

func RunEtcd(ctx context.Context, config *c.Config, etcdRunner etcdProcess, s3 s3client.Client) error {
	defer config.Logger.Sync()

	// always clean out data
	// data can be recreated from cluster
	// data restore is needed on full cluster restart
	if err := clearExistingData(config); err != nil {
		return err
	}

	// wait for existing cluster (and quorum)
	clusterCtx, clusterCancel := context.WithTimeout(ctx, time.Duration(config.InitialClusterTimeout))
	defer clusterCancel()

	client, err := etcdclient.NewClientFromPeers(clusterCtx, config)
	if err != nil {
		// no members found
		config.Logger.Info("no members found")

		// attempt restoring backup
		verifyS3Ctx, verifyS3Cancel := context.WithTimeout(ctx, config.S3VerifyTimeout)
		defer verifyS3Cancel()
		// if backup bucket can't be verified, fail instead of moving to new cluster
		if err := s3.Verify(verifyS3Ctx, config); err != nil {
			config.Logger.Error("failed to verify backup S3 resource", zap.Error(err))
			return err
		}
		ok, err := backup.RestoreSnapshot(ctx, config, s3, restoreVersionBump)
		if err != nil {
			return err
		}
		// backup resource accessible but no backups found. move on to new cluster from scratch
		if !ok {
			config.Logger.Info("starting member new fresh")
			return etcdRunner.StartNew(config)
		}

		config.Logger.Info("starting member existing with backup data")
		return etcdRunner.StartExisting(config)
	}
	defer client.Close()

	config.Logger.Info("existing members found")
	// found members - check if quorum is established
	if err := client.GetQuorum(clusterCtx); err != nil {
		config.Logger.Info("no quorum found")

		config.Logger.Info("starting member existing")
		return etcdRunner.StartExisting(config)
	}

	config.Logger.Info("quorum found")
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
	return etcdRunner.StartExisting(config)
}

func clearExistingData(config *c.Config) error {
	if d, ok := config.Env["ETCD_DATA_DIR"]; ok && d != "" {
		if err := removeDir(d); err != nil {
			config.Logger.Error("remove data dir", zap.Error(err))
			return err
		}
	}
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
