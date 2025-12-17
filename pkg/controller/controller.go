package controller

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	e "github.com/randomcoww/etcd-wrapper/pkg/controller/errors"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
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
			case nil, e.ErrNoCluster:
				continue
			default:
				return err
			}
		}
	}
	return nil
}

func (c *Controller) runEtcd(config *c.Config) error {
	ok, err := etcdprocess.DataExists(config)
	if err != nil {
		config.Logger.Error("data dir check failed", zap.Error(err))
		return err
	}
	if !ok {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(config.PeerTimeout)) // wait for existing cluster
		if _, err = etcdclient.NewClientFromPeers(ctx, config); err != nil {                   // no cluster found, go through new cluster steps
			err = c.restoreSnapshot(config)
			switch err {
			case nil, e.ErrNoBackup:
			default:
				return err
			}
		}
	}
	return c.P.Start()
}

func (c *Controller) runNode(config *c.Config) error {
	peerCtx, _ := context.WithTimeout(context.Background(), time.Duration(config.PeerTimeout))
	client, err := etcdclient.NewClientFromPeers(peerCtx, config)
	if err != nil {
		config.Logger.Error("get cluster failed", zap.Error(err))
		return e.ErrNoCluster
	}
	statusCtx, _ := context.WithTimeout(context.Background(), time.Duration(config.StatusTimeout))
	status, err := client.Status(statusCtx, config.ListenClientURLs[0])
	if err != nil {
		config.Logger.Error("get local node status failed", zap.Error(err))
		return e.ErrLocalNode
	}
	config.Logger.Info("health check success")

	if err := client.Defragment(statusCtx, config.ListenClientURLs[0]); err != nil {
		config.Logger.Error("run defragment failed", zap.Error(err))
		return e.ErrDefragment
	}
	config.Logger.Info("defragment success")

	if status.GetHeader().GetMemberId() != status.GetLeader() { // check if leader
		return nil
	}
	config.Logger.Info("local node is leader")

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
	snapshotFile, err := os.CreateTemp("", "snapshot*.db")
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
