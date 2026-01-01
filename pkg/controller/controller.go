package controller

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.uber.org/zap"
	"io"
	"os"
	"time"
)

type Controller struct {
	P        etcdprocess.EtcdProcess
	S3Client s3client.Client
}

func (c *Controller) RunEtcd(config *c.Config) error {
	return c.runEtcd(config)
}

func (c *Controller) runEtcd(config *c.Config) error {
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
	clusterCtx, _ := context.WithTimeout(context.Background(), time.Duration(config.ClusterTimeout))
	client, err := etcdclient.NewClientFromPeers(clusterCtx, config)
	if err != nil {
		// no cluster found, go through new cluster steps
		ok, err := c.restoreSnapshot(config)
		if err != nil {
			return err
		}
		if !ok {
			// start etcd in new state
			config.Logger.Info("start new cluster")
			return c.P.StartNew()
		}

		// cluster with quorum found
	} else {
		defer client.Close()

		clientCtx, _ := context.WithTimeout(context.Background(), time.Duration(config.ReplaceTimeout))
		listResp, err := client.MemberList(clientCtx)
		if err != nil {
			config.Logger.Error("list member failed", zap.Error(err))
			return err
		}
		localMember := findLocalMember(listResp, config)

		// join cluster
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
			listResp, err = client.MemberAdd(clientCtx, config.InitialAdvertisePeerURLs)
			if err != nil {
				config.Logger.Error("add member failed", zap.Error(err))
				return err
			}
			config.Logger.Info("added local member")
		}
	}

	// start etcd in existing state
	config.Logger.Info("start existing cluster")
	return c.P.StartExisting()
}

func (c *Controller) restoreSnapshot(config *c.Config) (bool, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(config.RestoreTimeout))
	dir, err := os.MkdirTemp("", "etcd-wrapper-*")
	if err != nil {
		config.Logger.Error("create path for snapshot failed", zap.Error(err))
		return false, err
	}
	defer os.RemoveAll(dir)

	snapshotFile, err := os.CreateTemp(dir, "snapshot-restore-*.db")
	if err != nil {
		config.Logger.Error("open file for snapshot failed", zap.Error(err))
		return false, err
	}
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()
	config.Logger.Info("opened file for snapshot")

	ok, err := c.S3Client.Download(ctx, config, func(ctx context.Context, reader io.Reader) error {
		b, err := io.Copy(snapshotFile, reader)
		if err != nil {
			return err
		}
		if b == 0 {
			return fmt.Errorf("download size was 0")
		}
		return nil
	})
	if err != nil {
		config.Logger.Error("download snapshot failed", zap.Error(err))
		return false, err
	}
	if !ok {
		config.Logger.Info("backup not found")
		return false, nil
	}
	if err := etcdprocess.RestoreV3Snapshot(ctx, config, snapshotFile.Name()); err != nil {
		config.Logger.Error("restore snapshot failed", zap.Error(err))
		return false, err
	}
	config.Logger.Info("restored backup")
	return true, nil
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
