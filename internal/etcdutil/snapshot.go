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

func RestoreSnapshot(snapshotFile string, logger *zap.Logger, peerURLs []string, dataDir, walDir, name, initialCluster, initialClusterToken string, revisionBump uint64) error {
	sp := snapshot.NewV3(logger)
	restoreConfig := snapshot.RestoreConfig{
		SnapshotPath:        snapshotFile,
		PeerURLs:            peerURLs,
		OutputDataDir:       dataDir,
		OutputWALDir:        walDir,
		Name:                name,
		InitialCluster:      initialCluster,
		InitialClusterToken: initialClusterToken,
		RevisionBump:        revisionBump,
		SkipHashCheck:       false,
	}
	if revisionBump > 0 {
		restoreConfig.MarkCompacted = true
	}
	if err := sp.Restore(restoreConfig); err != nil {
		return fmt.Errorf("restore snapshot from %s: %w", snapshotFile, err)
	}
	return nil
}
