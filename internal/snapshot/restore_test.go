package snapshot

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	baseTestPath string = "../../test/outputs"
)

type fakeS3Client struct {
	objectsList   []string
	objectsErrors map[string]error
	snapshotFiles map[string]string
}

func (c *fakeS3Client) VerifyBucket(ctx context.Context) (bool, error) {
	return true, nil
}

func (c *fakeS3Client) Download(ctx context.Context, key string, handler func(context.Context, io.Reader) error) (bool, error) {
	file, err := os.Open(c.snapshotFiles[key])
	if err != nil {
		return false, err
	}
	defer file.Close()
	return true, handler(ctx, file)
}

func (c *fakeS3Client) List(ctx context.Context, prefix string) ([]string, map[string]error) {
	return c.objectsList, c.objectsErrors
}

func TestRestore(t *testing.T) {
	dataPath := t.TempDir()

	fakeSnapshotFile6, _ := os.CreateTemp(dataPath, "snap-5")
	defer os.RemoveAll(fakeSnapshotFile6.Name())
	defer fakeSnapshotFile6.Close()
	if _, err := fakeSnapshotFile6.Write([]byte("bad data")); err != nil {
		t.Fatal("failed to write to temp file: %w", err)
	}

	fakeSnapshotFile4, _ := os.CreateTemp(dataPath, "snap-3")
	defer os.RemoveAll(fakeSnapshotFile4.Name())
	defer fakeSnapshotFile4.Close()

	s3Client := &fakeS3Client{
		objectsList: []string{
			"snap-2",
			"snap-3",
			"snap-4",
			"snap-5",
			"snap-6",
		},
		objectsErrors: make(map[string]error),
		snapshotFiles: map[string]string{
			"snap-2": filepath.Join(baseTestPath, "../rev2-snap.db"), // good data
			"snap-3": filepath.Join(baseTestPath, "../rev3-snap.db"), // good data
			"snap-4": fakeSnapshotFile4.Name(),                       // zero length
			"snap-5": "",                                             // non existent
			"snap-6": fakeSnapshotFile6.Name(),                       // bad data
		},
	}

	restoreDataPath := t.TempDir()
	logger, _ := zap.NewProduction()
	restoreConfig := NewRestoreConfig(&c.EnvConfig{
		Env: map[string]string{
			"ETCD_NAME":                  "node0",
			"ETCD_DATA_DIR":              restoreDataPath,
			"ETCD_INITIAL_CLUSTER":       "node0=https://127.0.0.1:8080,node1=https://127.0.0.2:8080,node2=https://127.0.0.3:8080",
			"ETCD_INITIAL_CLUSTER_TOEKN": "test-cluster-1",
		},
		InitialAdvertisePeerURLs: []string{"https://127.0.0.1:8080"},
	}, "", 1000)

	clientCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	ok, err := Restore(clientCtx, logger, s3Client, "snap-", restoreConfig)
	assert.NoError(t, err)
	assert.True(t, ok)

	revision, err := etcdutil.GetDataRevision(restoreDataPath, logger)
	assert.NoError(t, err)
	assert.Equal(t, int64(1003), revision) // should restore rev3 + revision bump
}
