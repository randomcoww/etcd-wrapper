package runner

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcd"
	s "github.com/randomcoww/etcd-wrapper/internal/snapshot"
	"github.com/randomcoww/etcd-wrapper/s3test"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestBackupRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, _ := zap.NewProduction()
	members := []member{
		{
			name:        "node0",
			clientPort:  8080,
			peerPort:    8090,
			metricsPort: 8100,
		},
		{
			name:        "node1",
			clientPort:  8081,
			peerPort:    8091,
			metricsPort: 8101,
		},
		{
			name:        "node2",
			clientPort:  8082,
			peerPort:    8092,
			metricsPort: 8102,
		},
	}
	dataPath := t.TempDir()
	configs, err := newConfigs(t, members, dataPath)
	if err != nil {
		t.Fatal("generate test configs: %w", err)
	}

	s3Client := &s3test.FakeS3Client{
		ObjectsList: []string{
			"snap-2",
		},
		ObjectsErrors: make(map[string]error),
		SnapshotFiles: map[string]string{
			"snap-2": filepath.Join(baseTestPath, "../rev2-snap.db"), // good data
		},
		TestEncryptionKey: testEncryptionKey,
	}

	var runners []*Runner
	var backupRunners []*Backup

	for _, config := range configs {
		snapshot := s.NewFromConfig(logger, s3Client, &c.YamlConfig{
			SnapshotEncryptionKey:  testEncryptionKey,
			SnapshotObjectPrefix:   "snap-",
			SnapshotRetentionCount: 2,
			S3ClientTimeout:        2 * time.Second,
			RestoreSnapshotTimeout: 2 * time.Second,
			UploadSnapshotTimeout:  2 * time.Second,
		}, func() string { return "test" })

		etcdRunner, err := etcd.NewEtcdRunner(etcdBinary)
		if err != nil {
			t.Fatal("test etcd binary: %w", err)
		}

		r := NewRunnerFromConfig(logger, etcdRunner, snapshot, &c.YamlConfig{
			CheckQuorumDelay: 8 * time.Second,
			MemberTimeout:    8 * time.Second,
		}, config)
		defer r.etcdRunner.Stop()
		runners = append(runners, r)

		b := NewBackupFromConfig(logger, snapshot, &c.YamlConfig{
			BackupInterval:    4 * time.Second,
			CheckQuorumDelay:  8 * time.Second,
			MemberTimeout:     4 * time.Second,
			SnapshotTimeout:   8 * time.Second,
			DefragmentTimeout: 16 * time.Second,
			DefragThreshold:   0.0,
		}, config)
		backupRunners = append(backupRunners, b)
	}

	if err := existingDummyCluster(t, ctx, runners); err != nil {
		t.Fatal("create dummy cluster: %w", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, len(backupRunners))

	for i, b := range backupRunners {
		wg.Add(1)
		backupCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		go func(index int) {
			errs[index] = b.run(backupCtx)
			wg.Done()
		}(i)
	}

	wg.Wait()
	for _, err := range errs {
		assert.NoError(t, err)
	}

	_, ok := s3Client.SnapshotFiles["snap-test"] // should create backup under SnapshotFiles
	assert.True(t, ok)
}
