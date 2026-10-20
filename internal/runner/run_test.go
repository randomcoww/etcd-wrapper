package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcd"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	s "github.com/randomcoww/etcd-wrapper/internal/snapshot"
	"github.com/randomcoww/etcd-wrapper/internal/tlsutil"
	"github.com/randomcoww/etcd-wrapper/s3test"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	baseTestPath      string = "../../test/outputs"
	testEncryptionKey string = "test-snapshot-encryption"
	etcdBinary        string = "/etcd/usr/local/bin/etcd"
)

type member struct {
	name        string
	clientPort  int
	peerPort    int
	metricsPort int
}

func TestCreateFromSnapshot(t *testing.T) {
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
	configs, err := newConfigs(t, members)
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
		defer etcdRunner.Stop()

		r := NewRunnerFromConfig(logger, etcdRunner, snapshot, &c.YamlConfig{
			CheckQuorumTimeout: 8 * time.Second,
			MemberTimeout:      8 * time.Second,
		}, config)
		runners = append(runners, r)

		ok, err := r.StartInit(ctx)
		if err != nil {
			t.Fatal("call etcd runner: %w", err)
		}
		assert.True(t, ok)
		time.Sleep(4 * time.Second)
	}

	// --- verify quorum, nodes, and backup ---

	for _, r := range runners {
		c, cancel := context.WithTimeout(ctx, time.Duration(4*time.Second))
		defer cancel()

		err := verifyTestStatus(t, c, r)
		assert.NoError(t, err)

		val, err := verifyTestData(t, c, r, "test-rev2")
		assert.NoError(t, err)
		assert.Equal(t, "test-rev2-val", val) // match value that should exist in the test data
	}
}

func TestRestartMembers(t *testing.T) {
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
	configs, err := newConfigs(t, members)
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
		defer etcdRunner.Stop()

		r := NewRunnerFromConfig(logger, etcdRunner, snapshot, &c.YamlConfig{
			CheckQuorumTimeout: 8 * time.Second,
			MemberTimeout:      8 * time.Second,
		}, config)
		runners = append(runners, r)
	}

	if err := existingDummyCluster(t, ctx, runners, filepath.Join(baseTestPath, "../rev3-snap.db")); err != nil {
		t.Fatal("create dummy cluster: %w", err)
	}

	// --- stop two nodes ---

	t.Logf("stopping nodes")
	for _, r := range runners[:2] {
		if err := r.etcdRunner.Stop(); err != nil {
			t.Fatalf("wait stop test client: %v", err)
		}
		t.Logf("stopped %s", r.etcdConfig.Env["ETCD_NAME"])

		// remove data (data loss)
		if err := r.clearLocalData(); err != nil {
			t.Fatalf("test data loss, %v", err)
		}
	}

	// --- start replacement nodes ---

	for _, r := range runners[:2] {
		t.Logf("starting %s", r.etcdConfig.Env["ETCD_NAME"])

		ok, err := r.StartInit(ctx)
		if err != nil {
			t.Fatal("call etcd runner: %w", err)
		}
		assert.True(t, ok)
		time.Sleep(4 * time.Second)
	}

	// --- verify quorum, nodes, and backup ---

	for _, r := range runners {
		runCtx, cancel := context.WithTimeout(ctx, time.Duration(4*time.Second))
		defer cancel()

		err := verifyTestStatus(t, runCtx, r)
		assert.NoError(t, err)

		val, err := verifyTestData(t, runCtx, r, "test-rev2")
		assert.NoError(t, err)
		assert.Equal(t, "test-rev2-val", val) // match value that should exist in the test data
	}
}

func TestRollingMemberUpdate(t *testing.T) {
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
	membersAfter := []member{
		{
			name:        "node0", // update URLs
			clientPort:  8080,
			peerPort:    8090,
			metricsPort: 8100,
		},
		{
			name:        "node1",
			clientPort:  8083,
			peerPort:    8093,
			metricsPort: 8103,
		},
		{
			name:        "node2",
			clientPort:  8082,
			peerPort:    8092,
			metricsPort: 8102,
		},
	}

	configs, err := newConfigs(t, members)
	if err != nil {
		t.Fatal("generate test configs: %w", err)
	}
	configsAfter, err := newConfigs(t, membersAfter)
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
		defer etcdRunner.Stop()

		r := NewRunnerFromConfig(logger, etcdRunner, snapshot, &c.YamlConfig{
			CheckQuorumTimeout: 8 * time.Second,
			MemberTimeout:      8 * time.Second,
		}, config)
		runners = append(runners, r)
	}

	if err := existingDummyCluster(t, ctx, runners, filepath.Join(baseTestPath, "../rev3-snap.db")); err != nil {
		t.Fatal("create dummy cluster: %w", err)
	}

	// --- replace each node ---

	t.Logf("rolling restart")
	for i, r := range runners {
		if err := r.etcdRunner.Stop(); err != nil {
			t.Fatal("wait stop test client: %w", err)
		}

		// update config
		r.etcdConfig = configsAfter[i]

		t.Logf("restarting %s", r.etcdConfig.Env["ETCD_NAME"])

		ok, err := r.StartInit(ctx)
		if err != nil {
			t.Fatal("call etcd runner: %w", err)
		}
		assert.True(t, ok)
		time.Sleep(4 * time.Second)
	}

	// --- verify quorum, nodes, and backup ---

	for _, r := range runners {
		c, cancel := context.WithTimeout(ctx, time.Duration(4*time.Second))
		defer cancel()

		err := verifyTestStatus(t, c, r)
		assert.NoError(t, err)

		val, err := verifyTestData(t, c, r, "test-rev3")
		assert.NoError(t, err)
		assert.Equal(t, "test-rev3-val", val) // match value that should exist in the test data
	}
}

func TestStartNew(t *testing.T) {
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
	configs, err := newConfigs(t, members)
	if err != nil {
		t.Fatal("generate test configs: %w", err)
	}

	s3Client := &s3test.FakeS3Client{
		ObjectsList:       []string{},
		ObjectsErrors:     make(map[string]error),
		SnapshotFiles:     map[string]string{},
		TestEncryptionKey: testEncryptionKey,
	}

	var runners []*Runner
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
		defer etcdRunner.Stop()

		r := NewRunnerFromConfig(logger, etcdRunner, snapshot, &c.YamlConfig{
			CheckQuorumTimeout: 8 * time.Second,
			MemberTimeout:      8 * time.Second,
		}, config)
		runners = append(runners, r)

		ok, err := r.StartInit(ctx)
		if err != nil {
			t.Fatal("init: %w", err)
		}
		if !ok {
			t.Fatal("etcd new: %w", err)
		}
		time.Sleep(4 * time.Second)
	}

	// --- verify quorum, nodes ---

	for _, runner := range runners {
		c, cancel := context.WithTimeout(ctx, time.Duration(60*time.Second))
		defer cancel()

		err := verifyTestStatus(t, c, runner)
		assert.NoError(t, err)
	}
}

// --- helper ---

func verifyTestStatus(t *testing.T, ctx context.Context, r *Runner) error {
	t.Helper()

	memberClientURLs := memberClientURLs(t, r)
	client, err := etcdutil.NewClient(ctx, r.logger, memberClientURLs, r.etcdConfig.ClientTLSConfig)
	if err != nil {
		return err
	}
	if err := client.CheckQuorum(ctx); err != nil {
		return err
	}
	status, err := client.Status(ctx, memberClientURLs[0])
	if err != nil {
		return err
	}
	t.Logf("status: %+v", status)
	return nil
}

func verifyTestData(t *testing.T, ctx context.Context, r *Runner, key string) (string, error) {
	t.Helper()

	memberClientURLs := memberClientURLs(t, r)
	client, err := etcdutil.NewClient(ctx, r.logger, memberClientURLs, r.etcdConfig.ClientTLSConfig)
	if err != nil {
		return "", err
	}
	resp, err := client.C().KV.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return string(resp.Kvs[0].Value), nil
}

func newConfigs(t *testing.T, members []member) ([]*c.EtcdConfig, error) {
	t.Helper()

	var configs []*c.EtcdConfig
	for _, member := range members {
		var err error
		config := &c.EtcdConfig{
			Env: map[string]string{
				"ETCD_NAME":                        member.name,
				"ETCD_LOG_LEVEL":                   "error",
				"ETCD_LOG_OUTPUTS":                 "stdout",
				"ETCD_DATA_DIR":                    t.TempDir(),
				"ETCD_CLIENT_CERT_AUTH":            "true",
				"ETCD_PEER_CLIENT_CERT_AUTH":       "true",
				"ETCD_STRICT_RECONFIG_CHECK":       "true",
				"ETCD_TRUSTED_CA_FILE":             filepath.Join(baseTestPath, member.name, "client", "ca.crt"),
				"ETCD_CERT_FILE":                   filepath.Join(baseTestPath, member.name, "client", "tls.crt"),
				"ETCD_KEY_FILE":                    filepath.Join(baseTestPath, member.name, "client", "tls.key"),
				"ETCD_PEER_TRUSTED_CA_FILE":        filepath.Join(baseTestPath, member.name, "peer", "ca.crt"),
				"ETCD_PEER_CERT_FILE":              filepath.Join(baseTestPath, member.name, "peer", "tls.crt"),
				"ETCD_PEER_KEY_FILE":               filepath.Join(baseTestPath, member.name, "peer", "tls.key"),
				"ETCD_LISTEN_CLIENT_URLS":          fmt.Sprintf("https://127.0.0.1:%d", member.clientPort),
				"ETCD_ADVERTISE_CLIENT_URLS":       fmt.Sprintf("https://127.0.0.1:%d", member.clientPort),
				"ETCD_LISTEN_PEER_URLS":            fmt.Sprintf("https://127.0.0.1:%d", member.peerPort),
				"ETCD_INITIAL_ADVERTISE_PEER_URLS": fmt.Sprintf("https://127.0.0.1:%d", member.peerPort),
				"ETCD_LISTEN_METRICS_URLS":         fmt.Sprintf("http://127.0.0.1:%d", member.metricsPort),
				"ETCD_INITIAL_CLUSTER_TOKEN":       "test",
				"ETCD_AUTO_COMPACTION_RETENTION":   "1",
				"ETCD_AUTO_COMPACTION_MODE":        "revision",
				"ETCD_SOCKET_REUSE_ADDRESS":        "true",
				"ETCD_SOCKET_REUSE_PORT":           "true",
			},
			InitialAdvertisePeerURLs: []string{fmt.Sprintf("https://127.0.0.1:%d", member.peerPort)},
		}

		var initialCluster []string
		config.InitialCluster = make(map[string][]string)
		for _, member := range members {
			initialCluster = append(initialCluster, fmt.Sprintf("%s=https://127.0.0.1:%d", member.name, member.peerPort))
			config.InitialCluster[member.name] = []string{fmt.Sprintf("https://127.0.0.1:%d", member.peerPort)}
		}
		config.Env["ETCD_INITIAL_CLUSTER"] = strings.Join(initialCluster, ",")

		config.ClientTLSConfig, err = tlsutil.BuildTLSClientConfig(
			filepath.Join(baseTestPath, member.name, "client", "tls.crt"),
			filepath.Join(baseTestPath, member.name, "client", "tls.key"),
			[]string{filepath.Join(baseTestPath, member.name, "client", "ca.crt")},
		)
		if err != nil {
			return nil, err
		}
		config.PeerTLSConfig, err = tlsutil.BuildTLSClientConfig(
			filepath.Join(baseTestPath, member.name, "peer", "tls.crt"),
			filepath.Join(baseTestPath, member.name, "peer", "tls.key"),
			[]string{filepath.Join(baseTestPath, member.name, "peer", "ca.crt")},
		)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, nil
}

func existingDummyCluster(t *testing.T, ctx context.Context, runners []*Runner, snapshotPath string) error {
	t.Helper()

	for _, r := range runners {
		// preseed data into etcd path
		restoreConfig := r.etcdConfig.RestoreConfig()
		restoreConfig.RevisionBump = 0
		restoreConfig.SnapshotPath = snapshotPath

		if err := etcdutil.RestoreSnapshot(r.logger, restoreConfig); err != nil {
			return fmt.Errorf("dummy cluster data restore: %w", err)
		}
		if err := r.etcdRunner.StartExisting(ctx, r.etcdConfig.WriteEnv()); err != nil {
			return fmt.Errorf("dummy cluster: %w", err)
		}
	}
	time.Sleep(4 * time.Second)

	for _, r := range runners {
		c, cancel := context.WithTimeout(ctx, time.Duration(4*time.Second))
		defer cancel()

		if err := verifyTestStatus(t, c, r); err != nil {
			return fmt.Errorf("test dummy cluster: %w", err)
		}
	}
	return nil
}

func memberClientURLs(t *testing.T, r *Runner) []string {
	t.Helper()

	var urls []string
	if v, ok := r.etcdConfig.Env["ETCD_LISTEN_CLIENT_URLS"]; ok {
		for _, u := range strings.Split(v, ",") {
			urls = append(urls, u)
		}
	}
	return urls
}
