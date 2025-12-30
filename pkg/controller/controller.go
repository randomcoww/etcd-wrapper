package controller

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	e "github.com/randomcoww/etcd-wrapper/pkg/controller/errors"
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

func (c *Controller) RunNode(ctx context.Context, config *c.Config) error {
	for {
		timer := time.NewTimer(config.NodeRunInterval)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			err := c.runNode(config)
			switch err {
			case nil, e.ErrNoCluster, e.ErrLocalNode: // container healthcheck should terminate node under these conditions
				continue
			default:
				return err
			}
		}
	}
	return nil
}

func (c *Controller) runEtcd(config *c.Config) error {
	defer config.Logger.Sync()

	ok, err := etcdprocess.DataExists(config)
	if err != nil {
		config.Logger.Error("data dir check failed", zap.Error(err))
		return err
	}
	if !ok {
		clusterCtx, _ := context.WithTimeout(context.Background(), time.Duration(config.ClusterTimeout)) // wait for existing cluster
		client, err := etcdclient.NewClientFromPeers(clusterCtx, config)
		if err != nil { // no cluster found, go through new cluster steps
			err = c.restoreSnapshot(config)
			switch err {
			case nil:
			case e.ErrNoBackup:
				config.Logger.Info("start new cluster")
				return c.P.StartNew()
			default:
				return err
			}
		} else {
			defer client.Close()

			clientCtx, _ := context.WithTimeout(context.Background(), time.Duration(config.ReplaceTimeout))
			listResp, err := client.MemberList(clientCtx)
			if err != nil {
				config.Logger.Error("list member failed", zap.Error(err))
				return err
			}
			var localMember *etcdserverpb.Member
			for _, member := range listResp.GetMembers() {
				if util.HasMatchingElement(member.GetPeerURLs(), config.InitialAdvertisePeerURLs) {
					localMember = member
					break
				}
			}
			if localMember != nil && len(listResp.GetMembers()) >= len(config.ClusterPeerURLs) {
				listResp, err = client.MemberRemove(clientCtx, localMember.GetID())
				if err != nil {
					config.Logger.Error("remove member failed", zap.Error(err))
					return err
				}
				config.Logger.Info("removed local member")
				localMember = nil
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
	}
	config.Logger.Info("start existing cluster")
	return c.P.StartExisting()
}

func (c *Controller) runNode(config *c.Config) error {
	defer config.Logger.Sync()

	clusterCtx, _ := context.WithTimeout(context.Background(), time.Duration(config.ClusterTimeout))
	client, err := etcdclient.NewClientFromPeers(clusterCtx, config)
	if err != nil {
		config.Logger.Error("get cluster failed", zap.Error(err))
		return e.ErrNoCluster
	}
	defer client.Close()

	statusCtx, _ := context.WithTimeout(context.Background(), time.Duration(config.StatusTimeout))
	status, err := client.Status(statusCtx, config.LocalClientURL)
	if err != nil {
		config.Logger.Error("get local node status failed", zap.Error(err))
		return e.ErrLocalNode
	}
	config.Logger.Info("health check success")

	config.Logger.Info("node", zap.Int64("ID", int64(status.GetHeader().GetMemberId())))
	config.Logger.Info("leader", zap.Int64("ID", int64(status.GetLeader())))

	if err := client.Defragment(statusCtx, config.LocalClientURL); err != nil {
		config.Logger.Error("run defragment failed", zap.Error(err))
		return e.ErrDefragment
	}
	config.Logger.Info("defragment success")

	if status.GetHeader().GetMemberId() != status.GetLeader() { // continue if leader
		config.Logger.Info("skipping backup on non leader")
		return nil
	}

	uploadCtx, _ := context.WithTimeout(context.Background(), time.Duration(config.UploadTimeout))
	reader, err := client.Snapshot(uploadCtx)
	if err != nil {
		config.Logger.Error("create backup snapshot failed", zap.Error(err))
		return e.ErrCreateSnapshot
	}
	ok, err := c.S3Client.Upload(uploadCtx, config, reader)
	if err != nil {
		config.Logger.Error("upload backup snapshot failed", zap.Error(err))
		return e.ErrUploadBackup
	}
	if !ok {
		config.Logger.Error("snapshot file is empty")
		return e.ErrSnapshotEmpty
	}
	config.Logger.Info("created backup")

	return nil
}

func (c *Controller) restoreSnapshot(config *c.Config) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(config.RestoreTimeout))
	err := os.MkdirAll("/tmp/etcd-wrapper", 1777)
	if err != nil {
		config.Logger.Error("create path for snapshot failed", zap.Error(err))
		return err
	}
	snapshotFile, err := os.CreateTemp("/tmp/etcd-wrapper", "snapshot*.db")
	if err != nil {
		config.Logger.Error("open file for snapshot failed", zap.Error(err))
		return err
	}
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()

	ok, err := c.S3Client.Download(ctx, config, func(ctx context.Context, reader io.Reader) (bool, error) {
		b, err := io.Copy(snapshotFile, reader)
		if err != nil {
			return false, err
		}
		return b > 0, nil
	})
	if err != nil {
		config.Logger.Error("download snapshot failed", zap.Error(err))
		return e.ErrDownloadSnapshot
	}
	if !ok {
		config.Logger.Info("backup not found")
		return e.ErrNoBackup
	}
	if err := etcdprocess.RestoreV3Snapshot(ctx, config, snapshotFile.Name()); err != nil {
		config.Logger.Error("restore snapshot filed", zap.Error(err))
		return e.ErrRestoreSnapshot
	}
	config.Logger.Info("restored backup")
	return nil
}
