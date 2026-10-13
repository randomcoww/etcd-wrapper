package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	"github.com/randomcoww/etcd-wrapper/s3test"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	baseTestPath      string = "../../test/outputs"
	testEncryptionKey string = "test-snapshot-encryption"
)

func TestRestore(t *testing.T) {
	dataPath := t.TempDir()
	restoreDataPath := t.TempDir()

	fakeSnapshotFile6, _ := os.CreateTemp(dataPath, "snap-5")
	defer os.RemoveAll(fakeSnapshotFile6.Name())
	defer fakeSnapshotFile6.Close()
	if _, err := fakeSnapshotFile6.Write([]byte("bad data")); err != nil {
		t.Fatal("failed to write to temp file: %w", err)
	}

	fakeSnapshotFile4, _ := os.CreateTemp(dataPath, "snap-3")
	defer os.RemoveAll(fakeSnapshotFile4.Name())
	defer fakeSnapshotFile4.Close()

	envConfig := &c.EnvConfig{
		Env: map[string]string{
			"ETCD_NAME":                  "node0",
			"ETCD_DATA_DIR":              restoreDataPath,
			"ETCD_INITIAL_CLUSTER_TOEKN": "test-cluster-1",
		},
		InitialAdvertisePeerURLs: []string{"https://127.0.0.1:8080"},
		InitialCluster: map[string][]string{
			"node0": []string{"https://127.0.0.1:8080"},
			"node1": []string{"https://127.0.0.2:8080"},
			"node2": []string{"https://127.0.0.3:8080"},
		},
	}
	s3Client := &s3test.FakeS3Client{
		ObjectsList: []string{
			"snap-2",
			"snap-3",
			"snap-4",
			"snap-5",
			"snap-6",
		},
		ObjectsErrors: make(map[string]error),
		SnapshotFiles: map[string]string{
			"snap-2": filepath.Join(baseTestPath, "../rev2-snap.db"), // good data
			"snap-3": filepath.Join(baseTestPath, "../rev3-snap.db"), // good data
			"snap-4": fakeSnapshotFile4.Name(),                       // zero length
			"snap-5": "",                                             // non existent
			"snap-6": fakeSnapshotFile6.Name(),                       // bad data
		},
		TestEncryptionKey: testEncryptionKey,
	}

	logger, _ := zap.NewProduction()
	restoreConfig := envConfig.RestoreConfig("", 1000)

	clientCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	ok, err := Restore(clientCtx, logger, s3Client, "snap-", restoreConfig, testEncryptionKey)
	assert.NoError(t, err)
	assert.True(t, ok)

	revision, err := etcdutil.GetDataRevision(restoreDataPath, logger)
	assert.NoError(t, err)
	assert.Equal(t, int64(1003), revision) // should restore rev3 + revision bump
}
