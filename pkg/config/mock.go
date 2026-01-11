package config

import (
	"fmt"
	"go.uber.org/zap"
	"path/filepath"
	"strings"
)

const (
	baseTestPath string = "../../test/outputs"
)

func MockRunConfigs(dataPath, backupKeyPrefix string) []*Config {
	var (
		clientPortBase    int    = 8080
		peerPortBase      int    = 8090
		minioPort         int    = 9000
		minioBucket       string = "etcd"
		minioUser         string = "rootUser"
		minioPassword     string = "rootPassword"
		etcdBinaryFile    string = "/etcd/usr/local/bin/etcd"
		etcdutlBinaryFile string = "/etcd/usr/local/bin/etcdutl"
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
		"-etcd-binary-file", etcdBinaryFile,
		"-etcdutl-binary-file", etcdutlBinaryFile,
		"-s3-backup-resource-prefix", fmt.Sprintf("https://127.0.0.1:%d/%s/%s-", minioPort, minioBucket, backupKeyPrefix),
		"-s3-backup-ca-file", filepath.Join(baseTestPath, "minio", "certs", "CAs", "ca.crt"),
		"-s3-backup-count", "2",
		"-initial-cluster-timeout", "2s",
		"-restore-snapshot-timeout", "2s",
		"-member-replace-timeout", "8s",
		"-status-timeout", "8s",
		"-upload-snapshot-timeout", "8s",
		"-backup-interval", "8s",
	}

	var configs []*Config
	for i, member := range members {
		config := &Config{
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
				"AWS_ACCESS_KEY_ID":                minioUser,
				"AWS_SECRET_ACCESS_KEY":            minioPassword,
			},
		}
		config.parseArgs(append(commonArgs, "-local-client-url", fmt.Sprintf("https://127.0.0.1:%d", clientPortBase+i)))
		config.parseEnvs()
		configs = append(configs, config)
	}
	return configs
}
