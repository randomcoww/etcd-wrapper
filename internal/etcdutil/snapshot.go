package etcdutil

import (
	"fmt"
	"path/filepath"

	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.uber.org/zap"
)

func GetDataRevision(dataDir string, logger *zap.Logger) (int64, error) {
	sp := snapshot.NewV3(logger)
	status, err := sp.Status(filepath.Join(dataDir, "member", "snap", "db"))
	if err != nil {
		return 0, fmt.Errorf("get local revision: %w", err)
	}
	return status.Revision, nil
}

func RestoreSnapshot(logger *zap.Logger, restoreConfig snapshot.RestoreConfig) error {
	sp := snapshot.NewV3(logger)
	restoreConfig.MarkCompacted = restoreConfig.RevisionBump > 0
	if err := sp.Restore(restoreConfig); err != nil {
		return fmt.Errorf("restore snapshot: %w", err)
	}
	return nil
}
