package etcdutil

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.uber.org/zap"
)

const (
	baseTestPath string = "../../test/outputs"
)

func TestSnapshotRestore(t *testing.T) {
	dataPath := t.TempDir()
	logger, _ := zap.NewProduction()

	err := RestoreSnapshot(logger, snapshot.RestoreConfig{
		SnapshotPath:        filepath.Join(baseTestPath, "../rev3-snap.db"),
		Name:                "node0",
		OutputDataDir:       dataPath,
		PeerURLs:            []string{"https://127.0.0.1:8080"},
		InitialCluster:      "node0=https://127.0.0.1:8080,node1=https://127.0.0.2:8080,node2=https://127.0.0.3:8080",
		InitialClusterToken: "test-cluster-1",
		RevisionBump:        1000,
	})
	assert.NoError(t, err)

	revision, err := GetDataRevision(dataPath, logger)
	assert.NoError(t, err)

	assert.Equal(t, int64(1003), revision)
}
