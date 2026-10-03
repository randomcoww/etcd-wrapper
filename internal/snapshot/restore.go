package snapshot

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	"github.com/randomcoww/etcd-wrapper/internal/s3util"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.uber.org/zap"
)

func Restore(ctx context.Context, logger *zap.Logger, s3 s3util.Client, prefix string, restoreConfig snapshot.RestoreConfig) (bool, error) {
	keys, errors := s3.List(ctx, prefix)
	if len(keys) == 0 {
		return false, nil
	}
	for object, err := range errors {
		logger.Error("list", zap.String("object", object), zap.Error(err))
	}
	var ok bool
	var err error

	for i := len(keys) - 1; i >= 0; i-- {
		ok, err = restoreKey(ctx, logger, s3, keys[i], restoreConfig)
		if err == nil && ok {
			logger.Info("restored snapshot success", zap.String("key", keys[i]))
			break
		}
		logger.Error("restore failed", zap.String("key", keys[i]), zap.Error(err))
	}
	if err != nil {
		return false, fmt.Errorf("all restore failed %w", err)
	}
	return true, nil
}

func restoreKey(ctx context.Context, logger *zap.Logger, s3 s3util.Client, key string, restoreConfig snapshot.RestoreConfig) (bool, error) {
	logger.Info("attempting snapshot restore")
	dir, err := os.MkdirTemp("", "etcd-wrapper-*")
	if err != nil {
		logger.Error("create path for snapshot failed", zap.Error(err))
		return false, err
	}
	defer os.RemoveAll(dir)

	snapshotFile, err := os.CreateTemp(dir, "snapshot-restore-*.db")
	if err != nil {
		logger.Error("open file for snapshot failed", zap.Error(err))
		return false, err
	}
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()
	logger.Info("opened file for snapshot")

	ok, err := s3.Download(ctx, key, func(ctx context.Context, reader io.Reader) error {
		b, err := io.Copy(snapshotFile, reader)
		if err != nil {
			return err
		}
		if b == 0 {
			return fmt.Errorf("snapshot file download size was 0")
		}
		return nil
	})
	if err != nil {
		logger.Error("download snapshot failed", zap.Error(err))
		return false, err
	}
	if !ok {
		logger.Info("no snapshots found")
		return false, nil
	}

	restoreConfig.SnapshotPath = snapshotFile.Name()
	if err := etcdutil.RestoreSnapshot(logger, restoreConfig); err != nil {
		logger.Error("restore snapshot failed", zap.Error(err))
		return false, err
	}
	logger.Info("finished restoring snapshot")
	return true, nil
}
