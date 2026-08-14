package runner

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/tlsutil"
	"go.uber.org/zap"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	baseTestPath string = "../../test/outputs"
)

func mockRunConfigs(dataPath string) ([]*c.Config, error) {
	var (
		clientPortBase int = 8080
		peerPortBase   int = 8090
	)

	members := []string{
		"node0",
		"node1",
		"node2",
	}
	logger, _ := zap.NewProduction()

	var configs []*c.Config
	for i, member := range members {
		var err error
		config := &c.Config{
			Cmd:    "run",
			Logger: logger,
			Env: map[string]string{
				"ETCD_DATA_DIR":                    filepath.Join(dataPath, member+".etcd"),
				"ETCD_NAME":                        member,
				"ETCD_CLIENT_CERT_AUTH":            "true",
				"ETCD_PEER_CLIENT_CERT_AUTH":       "true",
				"ETCD_STRICT_RECONFIG_CHECK":       "true",
				"ETCD_TRUSTED_CA_FILE":             filepath.Join(baseTestPath, "ca.crt"),
				"ETCD_CERT_FILE":                   filepath.Join(baseTestPath, member, "client", "tls.crt"),
				"ETCD_KEY_FILE":                    filepath.Join(baseTestPath, member, "client", "tls.key"),
				"ETCD_PEER_TRUSTED_CA_FILE":        filepath.Join(baseTestPath, "peer-ca.crt"),
				"ETCD_PEER_CERT_FILE":              filepath.Join(baseTestPath, member, "peer", "tls.crt"),
				"ETCD_PEER_KEY_FILE":               filepath.Join(baseTestPath, member, "peer", "tls.key"),
				"ETCD_LISTEN_CLIENT_URLS":          fmt.Sprintf("https://127.0.0.1:%d", clientPortBase+i),
				"ETCD_ADVERTISE_CLIENT_URLS":       fmt.Sprintf("https://127.0.0.1:%d", clientPortBase+i),
				"ETCD_LISTEN_PEER_URLS":            fmt.Sprintf("https://127.0.0.1:%d", peerPortBase+i),
				"ETCD_INITIAL_ADVERTISE_PEER_URLS": fmt.Sprintf("https://127.0.0.1:%d", peerPortBase+i),
				"ETCD_INITIAL_CLUSTER_TOKEN":       "test",
				"ETCD_LOG_LEVEL":                   "error",
				"ETCD_AUTO_COMPACTION_RETENTION":   "1",
				"ETCD_AUTO_COMPACTION_MODE":        "revision",
				"ETCD_SOCKET_REUSE_ADDRESS":        "true",
			},
			LocalClientURL:           fmt.Sprintf("https://127.0.0.1:%d", clientPortBase+i),
			EtcdBinaryFile:           "/etcd/usr/local/bin/etcd",
			EtcdutlBinaryFile:        "/etcd/usr/local/bin/etcdutl",
			ClientTimeout:            8 * time.Second,
			RestoreTimeout:           2 * time.Second, // local mock
			InitialClusterTimeout:    2 * time.Second,
			InitialAdvertisePeerURLs: []string{fmt.Sprintf("https://127.0.0.1:%d", peerPortBase+i)},
		}

		var initialCluster []string
		for i, member := range members {
			initialCluster = append(initialCluster, fmt.Sprintf("%s=https://127.0.0.1:%d", member, peerPortBase+i))
			config.ClusterPeerURLs = append(config.ClusterPeerURLs, fmt.Sprintf("https://127.0.0.1:%d", peerPortBase+i))
		}
		config.Env["ETCD_INITIAL_CLUSTER"] = strings.Join(initialCluster, ",")

		config.ClientTLSConfig, err = tlsutil.TLSConfig([]string{filepath.Join(baseTestPath, "ca.crt")}, filepath.Join(baseTestPath, member, "client", "tls.crt"), filepath.Join(baseTestPath, member, "client", "tls.key"))
		if err != nil {
			return nil, err
		}
		config.PeerTLSConfig, err = tlsutil.TLSConfig([]string{filepath.Join(baseTestPath, "peer-ca.crt")}, filepath.Join(baseTestPath, member, "peer", "tls.crt"), filepath.Join(baseTestPath, member, "peer", "tls.key"))
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, nil
}

func mockBackupConfigs(dataPath string) ([]*c.Config, error) {
	var (
		clientPortBase int = 8080
		peerPortBase   int = 8090
	)

	members := []string{
		"node0",
		"node1",
		"node2",
	}
	logger, _ := zap.NewProduction()

	var configs []*c.Config
	for i, member := range members {
		var err error
		config := &c.Config{
			Cmd:               "backup",
			Logger:            logger,
			LocalClientURL:    fmt.Sprintf("https://127.0.0.1:%d", clientPortBase+i),
			EtcdutlBinaryFile: "/etcd/usr/local/bin/etcdutl",
			ClientTimeout:     8 * time.Second,
			UploadTimeout:     2 * time.Second, // local mock
		}

		for i, _ := range members {
			config.ClusterPeerURLs = append(config.ClusterPeerURLs, fmt.Sprintf("https://127.0.0.1:%d", peerPortBase+i))
		}
		config.ClientTLSConfig, err = tlsutil.TLSConfig([]string{filepath.Join(baseTestPath, "ca.crt")}, filepath.Join(baseTestPath, member, "client", "tls.crt"), filepath.Join(baseTestPath, member, "client", "tls.key"))
		if err != nil {
			return nil, err
		}
		config.PeerTLSConfig, err = tlsutil.TLSConfig([]string{filepath.Join(baseTestPath, "peer-ca.crt")}, filepath.Join(baseTestPath, member, "peer", "tls.crt"), filepath.Join(baseTestPath, member, "peer", "tls.key"))
		if err != nil {
			return nil, err
		}
		configs = append(configs, config)
	}
	return configs, nil
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
