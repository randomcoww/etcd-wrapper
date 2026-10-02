package etcdutil

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	baseTestPath string = "../../test/outputs"
)

func TestSnapshotRestore(t *testing.T) {
	dataPath := t.TempDir()
	logger, _ := zap.NewProduction()

	err := RestoreSnapshot(
		filepath.Join(baseTestPath, "../rev3-snap.db"),
		logger,
		[]string{"https://127.0.0.1:8080"},
		dataPath,
		"",
		"node0",
		"node0=https://127.0.0.1:8080,node1=https://127.0.0.2:8080,node2=https://127.0.0.3:8080",
		"test-cluster-1",
		1000,
	)
	assert.NoError(t, err)

	revision, err := GetDataRevision(dataPath, logger)
	assert.NoError(t, err)

	assert.Equal(t, int64(1003), revision)
}
