package runner

// import (
// 	"context"
// 	"fmt"
// 	"path/filepath"
// 	"strings"
// 	"testing"
// 	"time"

// 	c "github.com/randomcoww/etcd-wrapper/config"
// 	"github.com/randomcoww/etcd-wrapper/internal/etcd"
// 	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
// 	g "github.com/randomcoww/etcd-wrapper/internal/gossip"
// 	"github.com/randomcoww/etcd-wrapper/internal/tlsutil"
// 	"github.com/randomcoww/etcd-wrapper/s3test"
// 	"github.com/stretchr/testify/assert"
// 	"go.uber.org/zap"
// )

// const (
// 	baseTestPath      string = "../../test/outputs"
// 	testEncryptionKey string = "test-snapshot-encryption"
// )

// type member struct {
// 	name        string
// 	clientPort  int
// 	peerPort    int
// 	metricsPort int
// 	gossipPort  int
// }

// func TestBackup(t *testing.T) {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	logger, _ := zap.NewProduction()
// 	s3Client := &s3test.FakeS3Client{
// 		ObjectsList: []string{
// 			"snap-2",
// 		},
// 		ObjectsErrors: make(map[string]error),
// 		SnapshotFiles: map[string]string{
// 			"snap-2": filepath.Join(baseTestPath, "../rev2-snap.db"), // good data
// 		},
// 		TestEncryptionKey: testEncryptionKey,
// 	}
// 	members := []member{
// 		{
// 			name:        "node0",
// 			clientPort:  8080,
// 			peerPort:    8090,
// 			metricsPort: 8100,
// 		},
// 		{
// 			name:        "node1",
// 			clientPort:  8081,
// 			peerPort:    8091,
// 			metricsPort: 8101,
// 		},
// 		{
// 			name:        "node2",
// 			clientPort:  8082,
// 			peerPort:    8092,
// 			metricsPort: 8102,
// 		},
// 	}
// 	dataPath := t.TempDir()
// 	configs, err := newConfigs(t, members, dataPath)
// 	if err != nil {
// 		t.Fatal("generate test configs: %w", err)
// 	}

// 	var runners []*Runner
// 	for _, config := range configs {
// 		r := &Runner{
// 			logger:                 logger,
// 			checkQuorumTimeout:     8 * time.Second,
// 			memberTimeout:          8 * time.Second,
// 			restoreSnapshotTimeout: 2 * time.Second,
// 			etcdRunner: &etcd.Fork{
// 				Ctx:        ctx,
// 				EtcdBinary: "/etcd/usr/local/bin/etcd",
// 			},
// 			s3Client:              s3Client,
// 			etcdConfig:            config,
// 			snapshotObjectPrefix:  "snap-",
// 			snapshotEncryptionKey: testEncryptionKey,
// 		}
// 		defer r.etcdRunner.Wait()
// 		defer r.etcdRunner.Stop()
// 		runners = append(runners, r)
// 	}

// 	if err := newDummyCluster(t, ctx, runners); err != nil {
// 		t.Fatal("create dummy cluster: %w", err)
// 	}

// 	backup := &Backup{
// 		logger:                logger,
// 		checkQuorumTimeout:    8 * time.Second,
// 		memberTimeout:         8 * time.Second,
// 		s3Client:              s3Client,
// 		etcdConfig:            config,
// 		snapshotObjectPrefix:  "snap-",
// 		snapshotEncryptionKey: testEncryptionKey,
// 	}

// 	s, err := backup.status()
// }
