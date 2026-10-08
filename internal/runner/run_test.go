package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcd"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	"github.com/randomcoww/etcd-wrapper/internal/tlsutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var (
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

type member struct {
	name        string
	clientPort  int
	peerPort    int
	metricsPort int
}

func TestCreateFromRestore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, _ := zap.NewProduction()
	s3Client := &fakeS3Client{
		objectsList: []string{
			"snap-2",
		},
		objectsErrors: make(map[string]error),
		snapshotFiles: map[string]string{
			"snap-2": filepath.Join(baseTestPath, "../rev2-snap.db"), // good data
		},
	}
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

	var runners []*Runner
	for _, config := range configs {
		runner := &Runner{
			logger:           logger,
			checkQuorumDelay: 8 * time.Second,
			clientTimeout:    4 * time.Second,
			etcdRunner: &etcd.Fork{
				Ctx:        ctx,
				EtcdBinary: "/etcd/usr/local/bin/etcd",
			},
			s3Client:     s3Client,
			envConfig:    config,
			objectPrefix: "snap-",
		}
		defer runner.etcdRunner.Wait()
		defer runner.etcdRunner.Stop()
		runners = append(runners, runner)

		c, cancel := context.WithTimeout(ctx, time.Duration(8*time.Second))
		defer cancel()

		if err := runner.runInterval(c, 10000); err != nil {
			t.Fatal("call etcd runner: %w", err)
		}
		time.Sleep(4 * time.Second)
	}

	// --- verify quorum, nodes, and backup ---

	for _, runner := range runners {
		c, cancel := context.WithTimeout(ctx, time.Duration(4*time.Second))
		defer cancel()

		err := verifyTestStatus(t, c, runner)
		assert.NoError(t, err)

		val, err := verifyTestData(t, c, runner, "test-rev2")
		assert.NoError(t, err)
		assert.Equal(t, "test-rev2-val", val) // match value that should exist in the test data
	}
}

func TestReplaceMembers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, _ := zap.NewProduction()
	s3Client := &fakeS3Client{
		objectsList: []string{
			"snap-2",
		},
		objectsErrors: make(map[string]error),
		snapshotFiles: map[string]string{
			"snap-2": filepath.Join(baseTestPath, "../rev2-snap.db"), // good data
		},
	}
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

	var runners []*Runner
	for _, config := range configs {
		runner := &Runner{
			logger:           logger,
			checkQuorumDelay: 8 * time.Second,
			clientTimeout:    4 * time.Second,
			etcdRunner: &etcd.Fork{
				Ctx:        ctx,
				EtcdBinary: "/etcd/usr/local/bin/etcd",
			},
			s3Client:     s3Client,
			envConfig:    config,
			objectPrefix: "snap-",
		}
		defer runner.etcdRunner.Wait()
		defer runner.etcdRunner.Stop()
		runners = append(runners, runner)
	}

	if err := newDummyCluster(t, ctx, runners); err != nil {
		t.Fatal("create dummy cluster: %w", err)
	}

	// --- stop two nodes ---

	t.Logf("stopping nodes")
	for _, runner := range runners[:2] {
		if err := runner.etcdRunner.Stop(); err != nil {
			t.Fatal("stop test client: %w", err)
		}
		if err := runner.etcdRunner.Wait(); err != nil {
			t.Fatal("wait stop test client: %w", err)
		}
		t.Logf("stopped %s", runner.envConfig.Env["ETCD_NAME"])
	}

	// --- start replacement nodes ---

	for _, runner := range runners[:2] {
		c, cancel := context.WithTimeout(ctx, time.Duration(8*time.Second))
		defer cancel()

		t.Logf("starting %s", runner.envConfig.Env["ETCD_NAME"])
		if err := runner.runInterval(c, 10000); err != nil {
			t.Fatal("call etcd runner: %w", err)
		}
		time.Sleep(4 * time.Second)
	}

	// --- verify quorum, nodes, and backup ---

	for _, runner := range runners {
		c, cancel := context.WithTimeout(ctx, time.Duration(4*time.Second))
		defer cancel()

		err := verifyTestStatus(t, c, runner)
		assert.NoError(t, err)

		val, err := verifyTestData(t, c, runner, "test-rev3")
		assert.NoError(t, err)
		assert.Equal(t, "test-rev3-val", val) // match value that should exist in the test data
	}
}

func TestRollingMemberReplace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, _ := zap.NewProduction()
	s3Client := &fakeS3Client{
		objectsList: []string{
			"snap-3",
		},
		objectsErrors: make(map[string]error),
		snapshotFiles: map[string]string{
			"snap-3": filepath.Join(baseTestPath, "../rev3-snap.db"), // good data
		},
	}
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

	dataPath := t.TempDir()
	configs, err := newConfigs(t, members, dataPath)
	if err != nil {
		t.Fatal("generate test configs: %w", err)
	}
	configsAfter, err := newConfigs(t, membersAfter, dataPath)
	if err != nil {
		t.Fatal("generate test configs: %w", err)
	}

	var runners []*Runner
	for _, config := range configs {
		runner := &Runner{
			logger:           logger,
			checkQuorumDelay: 8 * time.Second,
			clientTimeout:    4 * time.Second,
			etcdRunner: &etcd.Fork{
				Ctx:        ctx,
				EtcdBinary: "/etcd/usr/local/bin/etcd",
			},
			s3Client:     s3Client,
			envConfig:    config,
			objectPrefix: "snap-",
		}
		defer runner.etcdRunner.Wait()
		defer runner.etcdRunner.Stop()
		runners = append(runners, runner)
	}

	if err := newDummyCluster(t, ctx, runners); err != nil {
		t.Fatal("create dummy cluster: %w", err)
	}

	// --- replace each node ---

	t.Logf("rolling restart")
	for i, runner := range runners {
		if err := runner.etcdRunner.Stop(); err != nil {
			t.Fatal("stop test client: %w", err)
		}
		if err := runner.etcdRunner.Wait(); err != nil {
			t.Fatal("wait stop test client: %w", err)
		}

		// update config
		runner.envConfig = configsAfter[i]

		t.Logf("restarting %s", runner.envConfig.Env["ETCD_NAME"])
		c, cancel := context.WithTimeout(ctx, time.Duration(8*time.Second))
		defer cancel()

		if err := runner.runInterval(c, 10000); err != nil {
			t.Fatal("call etcd runner: %w", err)
		}
		time.Sleep(4 * time.Second)
	}

	// --- verify quorum, nodes, and backup ---

	for _, runner := range runners {
		c, cancel := context.WithTimeout(ctx, time.Duration(4*time.Second))
		defer cancel()

		err := verifyTestStatus(t, c, runner)
		assert.NoError(t, err)

		val, err := verifyTestData(t, c, runner, "test-rev3")
		assert.NoError(t, err)
		assert.Equal(t, "test-rev3-val", val) // match value that should exist in the test data
	}
}

// --- helper ---

func verifyTestStatus(t *testing.T, ctx context.Context, r *Runner) error {
	t.Helper()

	memberClientURLs := memberClientURLs(t, r)
	client, err := etcdutil.NewClient(ctx, r.logger, memberClientURLs, r.envConfig.ClientTLSConfig)
	if err != nil {
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
	client, err := etcdutil.NewClient(ctx, r.logger, memberClientURLs, r.envConfig.ClientTLSConfig)
	if err != nil {
		return "", err
	}
	resp, err := client.C().KV.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return string(resp.Kvs[0].Value), nil
}

func newConfigs(t *testing.T, members []member, dataPath string) ([]*c.EnvConfig, error) {
	t.Helper()

	var configs []*c.EnvConfig
	for _, member := range members {
		var err error
		config := &c.EnvConfig{
			Env: map[string]string{
				"ETCD_NAME":                        member.name,
				"ETCD_LOG_LEVEL":                   "error",
				"ETCD_DATA_DIR":                    filepath.Join(dataPath, member.name+"_etcd"),
				"ETCD_CLIENT_CERT_AUTH":            "true",
				"ETCD_PEER_CLIENT_CERT_AUTH":       "true",
				"ETCD_STRICT_RECONFIG_CHECK":       "true",
				"ETCD_TRUSTED_CA_FILE":             filepath.Join(baseTestPath, "client", "ca.crt"),
				"ETCD_CERT_FILE":                   filepath.Join(baseTestPath, member.name, "client", "tls.crt"),
				"ETCD_KEY_FILE":                    filepath.Join(baseTestPath, member.name, "client", "tls.key"),
				"ETCD_PEER_TRUSTED_CA_FILE":        filepath.Join(baseTestPath, "peer", "ca.crt"),
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
				"ETCDCTL_API":                      "3",
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
			[]string{filepath.Join(baseTestPath, "client", "ca.crt")},
		)
		if err != nil {
			return nil, err
		}
		config.PeerTLSConfig, err = tlsutil.BuildTLSClientConfig(
			filepath.Join(baseTestPath, member.name, "peer", "tls.crt"),
			filepath.Join(baseTestPath, member.name, "peer", "tls.key"),
			[]string{filepath.Join(baseTestPath, "peer", "ca.crt")},
		)
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, nil
}

func newDummyCluster(t *testing.T, ctx context.Context, runners []*Runner) error {
	t.Helper()

	for _, r := range runners {
		// preseed data into etcd path
		if err := etcdutil.RestoreSnapshot(r.logger, r.envConfig.RestoreConfig(filepath.Join(baseTestPath, "../rev3-snap.db"), 0)); err != nil {
			return fmt.Errorf("dummy cluster data restore: %w", err)
		}
		if err := r.etcdRunner.StartExisting(r.envConfig.WriteEnv()); err != nil {
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
		val, err := verifyTestData(t, c, r, "test-rev3")
		if err != nil {
			return fmt.Errorf("test dummy data: %w", err)
		}
		if val != "test-rev3-val" {
			return fmt.Errorf("test dummy data")
		}
	}
	return nil
}

func memberClientURLs(t *testing.T, r *Runner) []string {
	t.Helper()

	var urls []string
	if v, ok := r.envConfig.Env["ETCD_LISTEN_CLIENT_URLS"]; ok {
		for _, u := range strings.Split(v, ",") {
			urls = append(urls, u)
		}
	}
	return urls
}
