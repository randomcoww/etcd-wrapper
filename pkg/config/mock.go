package config

import (
	"fmt"
	"go.uber.org/zap"
	"path/filepath"
	"strings"
	"time"
)

const (
	clientPortBase    int    = 8080
	peerPortBase      int    = 8090
	etcdBinaryFile    string = "/etcd/usr/local/bin/etcd"
	etcdutlBinaryFile string = "/etcd/usr/local/bin/etcdutl"
	baseTestPath      string = "../../test/outputs"
)

func MockConfigs(dataPath string) []*Config {
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

	var configs []*Config
	for i, member := range members {
		config := &Config{
			EtcdBinaryFile:    etcdBinaryFile,
			EtcdutlBinaryFile: etcdutlBinaryFile,
			S3BackupHost:      "https://test.internal",
			S3BackupBucket:    "bucket",
			S3BackupKey:       "path/key",
			Logger:            logger,
			ClusterTimeout:    20 * time.Second,
			RestoreTimeout:    4 * time.Second,
			ReplaceTimeout:    20 * time.Second,
			UploadTimeout:     4 * time.Second,
			StatusTimeout:     4 * time.Second,
			NodeRunInterval:   1 * time.Minute,
			LocalClientURL:    fmt.Sprintf("https://127.0.0.1:%d", clientPortBase+i),
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
		config.ParseEnvs()
		configs = append(configs, config)
	}
	return configs
}
