package config

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	var (
		member string = "node0"
	)

	t.Setenv("ETCD_NAME", "test")
	t.Setenv("ETCD_LISTEN_CLIENT_URLS", "https://10.1.0.1:9080,https://127.0.0.1:9080,https://10.0.0.1:9080")
	t.Setenv("ETCD_INITIAL_ADVERTISE_PEER_URLS", "https://10.0.0.1:8080")
	t.Setenv("ETCD_INITIAL_CLUSTER", "node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080")
	t.Setenv("ETCD_TRUSTED_CA_FILE", filepath.Join(baseTestPath, "ca-cert.pem"))
	t.Setenv("ETCD_CERT_FILE", filepath.Join(baseTestPath, member, "client", "cert.pem"))
	t.Setenv("ETCD_KEY_FILE", filepath.Join(baseTestPath, member, "client", "key.pem"))
	t.Setenv("ETCD_PEER_TRUSTED_CA_FILE", filepath.Join(baseTestPath, "peer-ca-cert.pem"))
	t.Setenv("ETCD_PEER_CERT_FILE", filepath.Join(baseTestPath, member, "peer", "cert.pem"))
	t.Setenv("ETCD_PEER_KEY_FILE", filepath.Join(baseTestPath, member, "peer", "key.pem"))
	t.Setenv("ETCD_DATA_DIR", "/data/test")
	t.Setenv("ETCD_INITIAL_CLUSTER_STATE", "new")

	c, err := NewConfig([]string{
		"etcd-wrapper",
		"-local-client-url",
		"https://127.0.0.1:9080",
		"-etcd-binary-file",
		"/path/etcd",
		"-etcdutl-binary-file",
		"/path/etcdutl",
		"-s3-backup-resource-prefix",
		"https://test-1.internal:9000/bucket-1/path/etcd-0.db",
		"-s3-backup-ca-file",
		filepath.Join(baseTestPath, "minio", "certs", "CAs", "ca.crt"),
		"-s3-backup-count",
		"3",
	})
	assert.NoError(t, err)

	assert.Equal(t, map[string]string{
		"ETCD_NAME":                        "test",
		"ETCD_LISTEN_CLIENT_URLS":          "https://10.1.0.1:9080,https://127.0.0.1:9080,https://10.0.0.1:9080",
		"ETCD_INITIAL_ADVERTISE_PEER_URLS": "https://10.0.0.1:8080",
		"ETCD_INITIAL_CLUSTER":             "node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080",
		"ETCD_CLIENT_CERT_AUTH":            "true",
		"ETCD_TRUSTED_CA_FILE":             filepath.Join(baseTestPath, "ca-cert.pem"),
		"ETCD_CERT_FILE":                   filepath.Join(baseTestPath, member, "client", "cert.pem"),
		"ETCD_KEY_FILE":                    filepath.Join(baseTestPath, member, "client", "key.pem"),
		"ETCD_PEER_CLIENT_CERT_AUTH":       "true",
		"ETCD_PEER_TRUSTED_CA_FILE":        filepath.Join(baseTestPath, "peer-ca-cert.pem"),
		"ETCD_PEER_CERT_FILE":              filepath.Join(baseTestPath, member, "peer", "cert.pem"),
		"ETCD_PEER_KEY_FILE":               filepath.Join(baseTestPath, member, "peer", "key.pem"),
		"ETCD_LOG_OUTPUTS":                 "stdout",
		"ETCD_ENABLE_V2":                   "false",
		"ETCD_STRICT_RECONFIG_CHECK":       "true",
		"ETCDCTL_API":                      "3",
		"ETCD_DATA_DIR":                    "/data/test",
	}, c.Env)
	assert.Equal(t, "/path/etcd", c.EtcdBinaryFile)
	assert.Equal(t, "/path/etcdutl", c.EtcdutlBinaryFile)
	assert.Equal(t, "test-1.internal:9000", c.S3BackupHost)
	assert.Equal(t, "bucket-1", c.S3BackupBucket)
	assert.Equal(t, "path/etcd-0.db", c.S3BackupKeyPrefix)
	assert.Equal(t, 3, c.S3BackupCount)
	assert.Equal(t, "https://127.0.0.1:9080", c.LocalClientURL)
	assert.Equal(t, []string{
		"https://10.0.0.1:8080",
	}, c.InitialAdvertisePeerURLs)
	assert.Equal(t, []string{
		"https://10.0.0.1:8080",
		"https://10.0.0.2:8080",
	}, c.ClusterPeerURLs)
	assert.Equal(t, []string{
		"ETCDCTL_API=3",
		"ETCD_CERT_FILE=" + filepath.Join(baseTestPath, member, "client", "cert.pem"),
		"ETCD_CLIENT_CERT_AUTH=true",
		"ETCD_DATA_DIR=/data/test",
		"ETCD_ENABLE_V2=false",
		"ETCD_INITIAL_ADVERTISE_PEER_URLS=https://10.0.0.1:8080",
		"ETCD_INITIAL_CLUSTER=node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080",
		"ETCD_KEY_FILE=" + filepath.Join(baseTestPath, member, "client", "key.pem"),
		"ETCD_LISTEN_CLIENT_URLS=https://10.1.0.1:9080,https://127.0.0.1:9080,https://10.0.0.1:9080",
		"ETCD_LOG_OUTPUTS=stdout",
		"ETCD_NAME=test",
		"ETCD_PEER_CERT_FILE=" + filepath.Join(baseTestPath, member, "peer", "cert.pem"),
		"ETCD_PEER_CLIENT_CERT_AUTH=true",
		"ETCD_PEER_KEY_FILE=" + filepath.Join(baseTestPath, member, "peer", "key.pem"),
		"ETCD_PEER_TRUSTED_CA_FILE=" + filepath.Join(baseTestPath, "peer-ca-cert.pem"),
		"ETCD_STRICT_RECONFIG_CHECK=true",
		"ETCD_TRUSTED_CA_FILE=" + filepath.Join(baseTestPath, "ca-cert.pem"),
	}, c.WriteEnv())
}
