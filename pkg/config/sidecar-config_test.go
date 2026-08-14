package config

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupConfig(t *testing.T) {
	var (
		baseTestPath string = "../../test/outputs"
		member       string = "node0"
	)

	t.Setenv("ETCD_INITIAL_CLUSTER", "node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080")
	t.Setenv("ETCD_TRUSTED_CA_FILE", filepath.Join(baseTestPath, "ca.crt"))
	t.Setenv("ETCD_CERT_FILE", filepath.Join(baseTestPath, member, "client", "tls.crt"))
	t.Setenv("ETCD_KEY_FILE", filepath.Join(baseTestPath, member, "client", "tls.key"))
	t.Setenv("ETCD_PEER_TRUSTED_CA_FILE", filepath.Join(baseTestPath, "peer-ca.crt"))
	t.Setenv("ETCD_PEER_CERT_FILE", filepath.Join(baseTestPath, member, "peer", "tls.crt"))
	t.Setenv("ETCD_PEER_KEY_FILE", filepath.Join(baseTestPath, member, "peer", "tls.key"))

	c, err := NewConfig("sidecar", []string{
		"-local-client-url", "https://127.0.0.1:9080",
		"-etcdutl-binary-file", "/path/etcdutl",
		"-s3-backup-resource-prefix", "https://test-1.internal:9000/bucket-1/path/etcd-0.db",
		"-s3-backup-trusted-ca-file", filepath.Join(baseTestPath, "minio", "certs", "CAs", "ca.crt"),
		"-s3-backup-count", "3",
		"-s3-verify-timeout", "1m",
	})
	assert.NoError(t, err)

	assert.Equal(t, map[string]string{
		"ETCD_INITIAL_CLUSTER":      "node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080",
		"ETCD_TRUSTED_CA_FILE":      filepath.Join(baseTestPath, "ca.crt"),
		"ETCD_CERT_FILE":            filepath.Join(baseTestPath, member, "client", "tls.crt"),
		"ETCD_KEY_FILE":             filepath.Join(baseTestPath, member, "client", "tls.key"),
		"ETCD_PEER_TRUSTED_CA_FILE": filepath.Join(baseTestPath, "peer-ca.crt"),
		"ETCD_PEER_CERT_FILE":       filepath.Join(baseTestPath, member, "peer", "tls.crt"),
		"ETCD_PEER_KEY_FILE":        filepath.Join(baseTestPath, member, "peer", "tls.key"),
		"ETCDCTL_API":               "3",
	}, c.Env)
	assert.Equal(t, "/path/etcdutl", c.EtcdutlBinaryFile)
	assert.Equal(t, "test-1.internal:9000", c.S3BackupHost)
	assert.Equal(t, "bucket-1", c.S3BackupBucket)
	assert.Equal(t, "path/etcd-0.db", c.S3BackupKeyPrefix)
	assert.Equal(t, 3, c.S3BackupCount)
	assert.Equal(t, 1*time.Minute, c.S3VerifyTimeout)
	assert.Equal(t, "https://127.0.0.1:9080", c.LocalClientURL)
	assert.Equal(t, []string{
		"ETCDCTL_API=3",
		"ETCD_CERT_FILE=" + filepath.Join(baseTestPath, member, "client", "tls.crt"),
		"ETCD_INITIAL_CLUSTER=node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080",
		"ETCD_KEY_FILE=" + filepath.Join(baseTestPath, member, "client", "tls.key"),
		"ETCD_PEER_CERT_FILE=" + filepath.Join(baseTestPath, member, "peer", "tls.crt"),
		"ETCD_PEER_KEY_FILE=" + filepath.Join(baseTestPath, member, "peer", "tls.key"),
		"ETCD_PEER_TRUSTED_CA_FILE=" + filepath.Join(baseTestPath, "peer-ca.crt"),
		"ETCD_TRUSTED_CA_FILE=" + filepath.Join(baseTestPath, "ca.crt"),
	}, c.WriteEnv())
}
