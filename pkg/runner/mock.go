package runner

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"go.uber.org/zap"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	baseTestPath string = "../../test/outputs"
)

func mockConfigs(dataPath string) []*c.Config {
	var (
		clientPortBase int = 8080
		peerPortBase   int = 8090
	)

	members := []string{
		"node0",
		"node1",
		"node2",
	}
	var initialCluster []string
	for i, member := range members {
		initialCluster = append(initialCluster, fmt.Sprintf("%s=https://127.0.0.1:%d", member, peerPortBase+i))
	}
	logger, _ := zap.NewProduction()

	commonArgs := []string{
		"etcd-wrapper",
		"-etcd-binary-file", "/etcd/usr/local/bin/etcd",
		"-etcdutl-binary-file", "/etcd/usr/local/bin/etcdutl",
		"-initial-cluster-timeout", "2s",
		"-restore-snapshot-timeout", "2s",
		"-member-replace-timeout", "8s",
		"-status-timeout", "8s",
		"-upload-snapshot-timeout", "8s",
		"-backup-interval", "8s",
	}

	var configs []*c.Config
	for i, member := range members {
		config := &c.Config{
			Logger: logger,
			Env: map[string]string{
				"ETCD_DATA_DIR":                    filepath.Join(dataPath, member+".etcd"),
				"ETCD_NAME":                        member,
				"ETCD_CLIENT_CERT_AUTH":            "true",
				"ETCD_PEER_CLIENT_CERT_AUTH":       "true",
				"ETCD_STRICT_RECONFIG_CHECK":       "true",
				"ETCD_TRUSTED_CA_FILE":             filepath.Join(baseTestPath, "ca-cert.pem"),
				"ETCD_CERT_FILE":                   filepath.Join(baseTestPath, member, "client", "cert.pem"),
				"ETCD_KEY_FILE":                    filepath.Join(baseTestPath, member, "client", "key.pem"),
				"ETCD_PEER_TRUSTED_CA_FILE":        filepath.Join(baseTestPath, "peer-ca-cert.pem"),
				"ETCD_PEER_CERT_FILE":              filepath.Join(baseTestPath, member, "peer", "cert.pem"),
				"ETCD_PEER_KEY_FILE":               filepath.Join(baseTestPath, member, "peer", "key.pem"),
				"ETCD_LISTEN_CLIENT_URLS":          fmt.Sprintf("https://127.0.0.1:%d", clientPortBase+i),
				"ETCD_ADVERTISE_CLIENT_URLS":       fmt.Sprintf("https://127.0.0.1:%d", clientPortBase+i),
				"ETCD_LISTEN_PEER_URLS":            fmt.Sprintf("https://127.0.0.1:%d", peerPortBase+i),
				"ETCD_INITIAL_ADVERTISE_PEER_URLS": fmt.Sprintf("https://127.0.0.1:%d", peerPortBase+i),
				"ETCD_INITIAL_CLUSTER":             strings.Join(initialCluster, ","),
				"ETCD_INITIAL_CLUSTER_TOKEN":       "test",
				"ETCD_LOG_LEVEL":                   "error",
				"ETCD_AUTO_COMPACTION_RETENTION":   "1",
				"ETCD_AUTO_COMPACTION_MODE":        "revision",
				"ETCD_SOCKET_REUSE_ADDRESS":        "true",
			},
		}
		config.ParseArgs(append(commonArgs, "-local-client-url", fmt.Sprintf("https://127.0.0.1:%d", clientPortBase+i)))
		config.ParseEnvs()
		configs = append(configs, config)
	}
	return configs
}

type mockS3 struct{}

func (c *mockS3) Verify(ctx context.Context, config *c.Config) error {
	return nil
}

func (m *mockS3) Download(ctx context.Context, config *c.Config, key string, handler func(context.Context, io.Reader) error) (bool, error) {
	file, err := os.Open(filepath.Join(baseTestPath, "../test-snapshot.db"))
	if err != nil {
		return false, err
	}
	defer file.Close()
	return true, handler(ctx, file)
}

func (c *mockS3) Upload(ctx context.Context, config *c.Config, key string, reader io.Reader) error {
	return nil
}

func (c *mockS3) Remove(ctx context.Context, config *c.Config, keys []string) error {
	return nil
}

func (c *mockS3) List(ctx context.Context, config *c.Config) []string {
	return []string{
		"dummy", // just need non-zero keys
	}
}

type mockS3NoBackup struct{}

func (c *mockS3NoBackup) Verify(ctx context.Context, config *c.Config) error {
	return nil
}

func (m *mockS3NoBackup) Download(ctx context.Context, config *c.Config, key string, handler func(context.Context, io.Reader) error) (bool, error) {
	return false, nil
}

func (c *mockS3NoBackup) Upload(ctx context.Context, config *c.Config, key string, reader io.Reader) error {
	return nil
}

func (c *mockS3NoBackup) Remove(ctx context.Context, config *c.Config, keys []string) error {
	return nil
}

func (c *mockS3NoBackup) List(ctx context.Context, config *c.Config) []string {
	return []string{}
}
