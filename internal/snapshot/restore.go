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

func VerifyBucket(ctx context.Context, s3Client s3util.Client) (bool, error) {
	return s3Client.VerifyBucket(ctx)
}

func Restore(ctx context.Context, logger *zap.Logger, s3Client s3util.Client, objectPrefix string, restoreConfig snapshot.RestoreConfig) (bool, error) {
	ok, err := VerifyBucket(ctx, s3Client)
	if err != nil {
		return false, fmt.Errorf("verify bucket: %w", err)
	}
	if !ok {
		return false, nil
	}

	keys, errors := s3Client.List(ctx, objectPrefix)
	if len(keys) == 0 {
		return false, nil
	}
	for object, err := range errors {
		logger.Error("list", zap.String("object", object), zap.Error(err))
	}

	for i := len(keys) - 1; i >= 0; i-- {
		ok, err = RestoreKey(ctx, logger, s3Client, keys[i], restoreConfig)
		if err == nil && ok {
			logger.Info("restored snapshot", zap.String("key", keys[i]))
			break
		}
		logger.Error("restore snapshot", zap.String("key", keys[i]), zap.Error(err))
	}
	if err != nil {
		return false, fmt.Errorf("restore snapshot: %w", err)
	}
	return true, nil
}

func RestoreKey(ctx context.Context, logger *zap.Logger, s3Client s3util.Client, key string, restoreConfig snapshot.RestoreConfig) (bool, error) {
	logger.Info("attempting snapshot restore")
	dir, err := os.MkdirTemp("", "etcd-wrapper-*")
	if err != nil {
		return false, fmt.Errorf("create path for snapshot: %w", err)
	}
	defer os.RemoveAll(dir)

	snapshotFile, err := os.CreateTemp(dir, "snapshot-restore-*.db")
	if err != nil {
		return false, fmt.Errorf("open file for snapshot: %w", err)
	}
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()
	logger.Info("opened file for snapshot")

	ok, err := s3Client.Download(ctx, key, func(ctx context.Context, reader io.Reader) error {
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
		return false, fmt.Errorf("download snapshot: %w", err)
	}
	if !ok {
		logger.Info("no snapshots found")
		return false, nil
	}

	restoreConfig.SnapshotPath = snapshotFile.Name()
	if err := etcdutil.RestoreSnapshot(logger, restoreConfig); err != nil {
		return false, fmt.Errorf("restore snapshot to etcd data: %w", err)
	}
	logger.Info("finished restoring snapshot")
	return true, nil
}
