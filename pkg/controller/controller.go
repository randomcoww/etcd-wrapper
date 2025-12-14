package controller

import (
	"bytes"
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
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
		if err := c.runNode(config); err != nil {
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
			return err
		}
		if !ok {
			ok, err = c.restoreSnapshot(config)
			if err != nil {
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
		return err
	}

	statusCtx, _ := context.WithTimeout(context.Background(), time.Duration(c.statusTimeout))
	status, err := client.Status(statusCtx, config.ListenClientURLs[0])
	if err != nil {
		return err
	}

	if status.GetHeader().GetMemberId() != status.GetLeader() { // check if leader
		return nil
	}

	uploadCtx, _ := context.WithTimeout(context.Background(), time.Duration(c.uploadTimeout))
	reader, err := client.Snapshot(uploadCtx)
	if err != nil {
		return err
	}

	buf := &bytes.Buffer{}
	b, err := io.Copy(buf, reader)
	if err != nil {
		return err
	}
	if err := c.S3Client.Upload(uploadCtx, config.S3BackupBucket, config.S3BackupKey, buf, b); err != nil {
		return err
	}
	return nil
}

func (c *Controller) restoreSnapshot(config *c.Config) (bool, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(c.restoreTimeout))
	snapshotFile, err := os.CreateTemp("", "snapshot*.db")
	if err != nil {
		return false, err
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
		return false, err
	}
	if !ok {
		return false, nil
	}
	if err := etcdprocess.RestoreV3Snapshot(ctx, config, snapshotFile.Name()); err != nil {
		return false, err
	}
	return true, nil
}
