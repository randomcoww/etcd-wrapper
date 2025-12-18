package config

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

const (
	baseTestPath string = "../../test/outputs"
	member       string = "node0"
)

func TestNewConfig(t *testing.T) {
	t.Setenv("ETCD_LISTEN_CLIENT_URLS", "https://node0-1:9080,https://node0-0:9080")
	t.Setenv("ETCD_INITIAL_ADVERTISE_PEER_URLS", "https://node0-1:8080,https://node0-0:8080")
	t.Setenv("ETCD_INITIAL_CLUSTER", "node0=https://node0-0:8080,node1=https://node1-0:8080")
	t.Setenv("ETCD_TRUSTED_CA_FILE", filepath.Join(baseTestPath, "ca-cert.pem"))
	t.Setenv("ETCD_CERT_FILE", filepath.Join(baseTestPath, member, "client", "cert.pem"))
	t.Setenv("ETCD_KEY_FILE", filepath.Join(baseTestPath, member, "client", "key.pem"))
	t.Setenv("ETCD_PEER_TRUSTED_CA_FILE", filepath.Join(baseTestPath, "peer-ca-cert.pem"))
	t.Setenv("ETCD_PEER_CERT_FILE", filepath.Join(baseTestPath, member, "peer", "cert.pem"))
	t.Setenv("ETCD_PEER_KEY_FILE", filepath.Join(baseTestPath, member, "peer", "key.pem"))
	t.Setenv("ETCD_DATA_DIR", "/data/test")

	c, err := NewConfig([]string{
		"etcd-wrapper",
		"-etcd-binary-file",
		"/path/etcd",
		"-etcdutl-binary-file",
		"/path/etcdutl",
		"-s3-backup-resource",
		"https://test-1.internal:9000/bucket-1/path-1/key-0.db",
		"-s3-backup-ca-file",
		filepath.Join(baseTestPath, "minio", "certs", "CAs", "ca.crt"),
	})
	assert.NoError(t, err)

	assert.Equal(t, map[string]string{
		"ETCD_LISTEN_CLIENT_URLS":          "https://node0-1:9080,https://node0-0:9080",
		"ETCD_INITIAL_ADVERTISE_PEER_URLS": "https://node0-1:8080,https://node0-0:8080",
		"ETCD_INITIAL_CLUSTER":             "node0=https://node0-0:8080,node1=https://node1-0:8080",
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
	assert.Equal(t, "test-1.internal:9000", c.S3BackupEndpoint)
	assert.Equal(t, "bucket-1", c.S3BackupBucket)
	assert.Equal(t, "path-1/key-0.db", c.S3BackupKey)
	assert.Equal(t, []string{
		"https://node0-0:9080",
		"https://node0-1:9080",
	}, c.ListenClientURLs)
	assert.Equal(t, []string{
		"https://node0-0:8080",
		"https://node0-1:8080",
	}, c.InitialAdvertisePeerURLs)
	assert.Equal(t, []string{
		"https://node0-0:8080",
		"https://node1-0:8080",
	}, c.ClusterPeerURLs)
	assert.Equal(t, []string{
		"ETCDCTL_API=3",
		"ETCD_CERT_FILE=" + filepath.Join(baseTestPath, member, "client", "cert.pem"),
		"ETCD_CLIENT_CERT_AUTH=true",
		"ETCD_DATA_DIR=/data/test",
		"ETCD_ENABLE_V2=false",
		"ETCD_INITIAL_ADVERTISE_PEER_URLS=https://node0-1:8080,https://node0-0:8080",
		"ETCD_INITIAL_CLUSTER=node0=https://node0-0:8080,node1=https://node1-0:8080",
		"ETCD_KEY_FILE=" + filepath.Join(baseTestPath, member, "client", "key.pem"),
		"ETCD_LISTEN_CLIENT_URLS=https://node0-1:9080,https://node0-0:9080",
		"ETCD_LOG_OUTPUTS=stdout",
		"ETCD_PEER_CERT_FILE=" + filepath.Join(baseTestPath, member, "peer", "cert.pem"),
		"ETCD_PEER_CLIENT_CERT_AUTH=true",
		"ETCD_PEER_KEY_FILE=" + filepath.Join(baseTestPath, member, "peer", "key.pem"),
		"ETCD_PEER_TRUSTED_CA_FILE=" + filepath.Join(baseTestPath, "peer-ca-cert.pem"),
		"ETCD_STRICT_RECONFIG_CHECK=true",
		"ETCD_TRUSTED_CA_FILE=" + filepath.Join(baseTestPath, "ca-cert.pem"),
	}, c.WriteEnv())
}
