package snapshot

import (
	"context"
	"crypto/rand"
	"io"
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
	timeFormat        string = "20060102-150405"
)

func TestRestore(t *testing.T) {
	restoreDataPath := t.TempDir()

	fakeSnapshotFile6, _ := os.CreateTemp("", "snap-5")
	defer os.RemoveAll(fakeSnapshotFile6.Name())
	defer fakeSnapshotFile6.Close()
	if _, err := fakeSnapshotFile6.Write([]byte("bad data")); err != nil {
		t.Fatal("failed to write to temp file: %w", err)
	}

	fakeSnapshotFile4, _ := os.CreateTemp("", "snap-3")
	defer os.RemoveAll(fakeSnapshotFile4.Name())
	defer fakeSnapshotFile4.Close()

	etcdConfig := &c.EtcdConfig{
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
			"snap-2": filepath.Join(baseTestPath, "../rev2-snap.db"),
			"snap-3": filepath.Join(baseTestPath, "../rev3-snap.db"),
			"snap-4": fakeSnapshotFile4.Name(), // zero length
			"snap-5": "",                       // non existent
			"snap-6": fakeSnapshotFile6.Name(), // bad data
		},
		TestEncryptionKey: testEncryptionKey,
	}

	logger, _ := zap.NewProduction()
	snapshot := &Snapshot{
		logger:                 logger,
		s3Client:               s3Client,
		encryptionPassword:     testEncryptionKey,
		objectPrefix:           "snap-",
		restoreRevisionBump:    1000,
		s3ClientTimeout:        2 * time.Second,
		restoreSnapshotTimeout: 2 * time.Second,
		uploadSnapshotTimeout:  8 * time.Second,
	}

	clientCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ok, err := snapshot.Restore(clientCtx, etcdConfig.RestoreConfig())
	assert.NoError(t, err)
	assert.True(t, ok)

	revision, err := etcdutil.GetDataRevision(restoreDataPath, logger)
	assert.NoError(t, err)
	assert.Equal(t, int64(snapshot.restoreRevisionBump+3), revision) // should restore rev3 + revision bump
}

func TestCreateBackup(t *testing.T) {
	s3Client := &s3test.FakeS3Client{
		TempDir:           t.TempDir(),
		SnapshotFiles:     make(map[string]string),
		TestEncryptionKey: testEncryptionKey,
	}

	logger, _ := zap.NewProduction()
	baseNow, _ := time.Parse("2006-01-02", "2000-01-01")
	snapshot := &Snapshot{
		logger:                 logger,
		s3Client:               s3Client,
		encryptionPassword:     testEncryptionKey,
		objectPrefix:           "snap-",
		objectTagFunc:          func() string { return baseNow.Add(time.Duration(1 * time.Minute)).Format(timeFormat) },
		s3ClientTimeout:        2 * time.Second,
		restoreSnapshotTimeout: 2 * time.Second,
		uploadSnapshotTimeout:  8 * time.Second,
	}

	file := io.NopCloser(io.LimitReader(rand.Reader, 2*1024*1024*1024))
	clientCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := snapshot.CreateBackup(clientCtx, file)
	assert.NoError(t, err)

	_, ok := s3Client.SnapshotFiles["snap-20000101-000100"] // should create content under SnapshotFiles
	assert.True(t, ok)
}

func TestRemoveOldBackups(t *testing.T) {
	s3Client := &s3test.FakeS3Client{
		ObjectsList: []string{
			"snap-2",
			"snap-3",
			"snap-4",
			"snap-5",
			"snap-6",
		},
		RemoveCalled: []string{},
	}
	logger, _ := zap.NewProduction()
	snapshot := &Snapshot{
		logger:                 logger,
		s3Client:               s3Client,
		retentionCount:         2,
		s3ClientTimeout:        2 * time.Second,
		restoreSnapshotTimeout: 2 * time.Second,
		uploadSnapshotTimeout:  2 * time.Second,
	}

	clientCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = snapshot.RemoveOldBackups(clientCtx)

	assert.Equal(t, []string{"snap-2", "snap-3", "snap-4"}, s3Client.RemoveCalled)
}
