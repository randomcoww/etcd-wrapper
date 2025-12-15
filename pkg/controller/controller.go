package controller

import (
	"bytes"
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
	P                  etcdprocess.EtcdProcess
	S3Client           s3client.Client
	peerTimeout        time.Duration
	restoreTimeout     time.Duration
	uploadTimeout      time.Duration
	statusTimeout      time.Duration
	backoffWaitBetween time.Duration
}

func (c *Controller) RunEtcd(config *c.Config) error {
	if err := c.runEtcd(config); err != nil {
		return err
	}
	defer c.P.Wait()
	defer c.P.Stop()

	return c.P.Wait()
}

func (c *Controller) RunNode(ctx context.Context, config *c.Config) error {
	for {
		err := c.runNode(config)
		switch err {
		case nil, e.ErrNoCluster:
			continue
		default:
			return err
		}

		timer := time.NewTimer(c.backoffWaitBetween)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			continue
		}
	}
	return nil
}

func (c *Controller) runEtcd(config *c.Config) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(c.peerTimeout))
	_, err := etcdclient.NewClientFromPeers(ctx, config)
	if err != nil {
		ok, err := etcdprocess.DataExists(config)
		if err != nil {
			config.Logger.Error("data dir check failed", zap.Error(err))
			return err
		}
		if !ok {
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
	peerCtx, _ := context.WithTimeout(context.Background(), time.Duration(c.peerTimeout))
	client, err := etcdclient.NewClientFromPeers(peerCtx, config)
	if err != nil {
		config.Logger.Error("get cluster failed", zap.Error(err))
		return e.ErrNoCluster
	}
	statusCtx, _ := context.WithTimeout(context.Background(), time.Duration(c.statusTimeout))
	status, err := client.Status(statusCtx, config.ListenClientURLs[0])
	if err != nil {
		config.Logger.Error("get local node status failed", zap.Error(err))
		return e.ErrLocalNode
	}
	config.Logger.Info("health check success")

	if status.GetHeader().GetMemberId() != status.GetLeader() { // check if leader
		return nil
	}
	config.Logger.Info("local node is leader")

	uploadCtx, _ := context.WithTimeout(context.Background(), time.Duration(c.uploadTimeout))
	reader, err := client.Snapshot(uploadCtx)
	if err != nil {
		config.Logger.Error("create backup snapshot failed", zap.Error(err))
		return e.ErrCreateSnapshot
	}
	buf := &bytes.Buffer{}
	b, err := io.Copy(buf, reader)
	if err != nil {
		config.Logger.Error("create snapshot failed", zap.Error(err))
		return err
	}
	if b == 0 {
		config.Logger.Error("snapshot file is zero length")
		return e.ErrSnapshotEmpty
	}
	if err := c.S3Client.Upload(uploadCtx, config.S3BackupBucket, config.S3BackupKey, buf, b); err != nil {
		config.Logger.Error("upload backup snapshot failed", zap.Error(err))
		return e.ErrUploadBackup
	}
	config.Logger.Info("created backup")

	return nil
}

func (c *Controller) restoreSnapshot(config *c.Config) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(c.restoreTimeout))
	snapshotFile, err := os.CreateTemp("", "snapshot*.db")
	if err != nil {
		config.Logger.Error("open file for snapshot failed", zap.Error(err))
		return err
	}
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()

	ok, err := c.S3Client.Download(ctx, config.S3BackupBucket, config.S3BackupKey, func(ctx context.Context, reader io.Reader) (bool, error) {
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
