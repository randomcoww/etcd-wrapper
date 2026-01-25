package runner

import (
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"go.uber.org/zap"
	"time"
)

const (
	timeFormat string = "20060102-150405"
)

func RunBackup(ctx context.Context, config *c.Config, s3 s3client.Client) error {
	defer config.Logger.Sync()

	// wait for existing cluster (and quorum)
	clusterCtx, clusterCancel := context.WithTimeout(ctx, time.Duration(config.ClusterTimeout))
	defer clusterCancel()

	client, err := etcdclient.NewClientFromPeers(clusterCtx, config)
	if err != nil {
		config.Logger.Error("get client failed", zap.Error(err))
		return err
	}
	defer client.Close()

	if err := client.GetQuorum(clusterCtx); err != nil {
		config.Logger.Error("get cluster failed", zap.Error(err))
		return err
	}

	statusCtx, statusCancel := context.WithTimeout(ctx, time.Duration(config.StatusTimeout))
	defer statusCancel()

	status, err := client.Status(statusCtx, config.LocalClientURL)
	if err != nil {
		config.Logger.Error("get local node status failed", zap.Error(err))
		return err
	}
	config.Logger.Info("local node responds to status")

	if err := client.Defragment(statusCtx, config.LocalClientURL); err != nil {
		config.Logger.Error("run defragment failed", zap.Error(err))
		return err
	}
	config.Logger.Info("defragment success")

	config.Logger.Info("node", zap.Int64("ID", int64(status.GetHeader().GetMemberId())))
	config.Logger.Info("leader", zap.Int64("ID", int64(status.GetLeader())))

	if status.GetHeader().GetMemberId() != status.GetLeader() {
		config.Logger.Info("skipping backup on non leader")
		return nil
	}

	// continue to run backup if leader
	uploadCtx, uploadCancel := context.WithTimeout(ctx, time.Duration(config.UploadTimeout))
	defer uploadCancel()

	reader, err := client.Snapshot(uploadCtx)
	if err != nil {
		config.Logger.Error("create backup snapshot failed", zap.Error(err))
		return err
	}
	if err := backup.UploadSnapshot(uploadCtx, config, s3, reader, func() string {
		return time.Now().Format(timeFormat)
	}); err != nil {
		config.Logger.Error("upload backup snapshot failed", zap.Error(err))
		return err
	}
	config.Logger.Info("created backup")

	return nil
}
