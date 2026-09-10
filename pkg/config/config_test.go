package config

import (
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func TestRunConfig(t *testing.T) {
	var (
		baseTestPath string = "../../test/outputs"
		member       string = "node0"
	)

	t.Setenv("ETCD_NAME", "test")
	t.Setenv("ETCD_LISTEN_CLIENT_URLS", "https://10.1.0.1:9080,https://127.0.0.1:9080,https://10.0.0.1:9080")
	t.Setenv("ETCD_INITIAL_ADVERTISE_PEER_URLS", "https://10.0.0.1:8080")
	t.Setenv("ETCD_INITIAL_CLUSTER", "node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080")
	t.Setenv("ETCD_TRUSTED_CA_FILE", filepath.Join(baseTestPath, "client", "ca.crt"))
	t.Setenv("ETCD_CERT_FILE", filepath.Join(baseTestPath, member, "client", "tls.crt"))
	t.Setenv("ETCD_KEY_FILE", filepath.Join(baseTestPath, member, "client", "tls.key"))
	t.Setenv("ETCD_PEER_TRUSTED_CA_FILE", filepath.Join(baseTestPath, "peer", "ca.crt"))
	t.Setenv("ETCD_PEER_CERT_FILE", filepath.Join(baseTestPath, member, "peer", "tls.crt"))
	t.Setenv("ETCD_PEER_KEY_FILE", filepath.Join(baseTestPath, member, "peer", "tls.key"))
	t.Setenv("ETCD_DATA_DIR", "/data/test")
	t.Setenv("ETCD_INITIAL_CLUSTER_STATE", "new")

	c, err := NewConfig([]string{
		"etcd-wrapper",
		"-local-client-url", "https://127.0.0.1:9080",
		"-etcd-binary-file", "/path/etcd",
	})
	assert.NoError(t, err)

	assert.Equal(t, map[string]string{
		"ETCD_NAME":                        "test",
		"ETCD_LISTEN_CLIENT_URLS":          "https://10.1.0.1:9080,https://127.0.0.1:9080,https://10.0.0.1:9080",
		"ETCD_INITIAL_ADVERTISE_PEER_URLS": "https://10.0.0.1:8080",
		"ETCD_INITIAL_CLUSTER":             "node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080",
		"ETCD_CLIENT_CERT_AUTH":            "true",
		"ETCD_TRUSTED_CA_FILE":             filepath.Join(baseTestPath, "client", "ca.crt"),
		"ETCD_CERT_FILE":                   filepath.Join(baseTestPath, member, "client", "tls.crt"),
		"ETCD_KEY_FILE":                    filepath.Join(baseTestPath, member, "client", "tls.key"),
		"ETCD_PEER_CLIENT_CERT_AUTH":       "true",
		"ETCD_PEER_TRUSTED_CA_FILE":        filepath.Join(baseTestPath, "peer", "ca.crt"),
		"ETCD_PEER_CERT_FILE":              filepath.Join(baseTestPath, member, "peer", "tls.crt"),
		"ETCD_PEER_KEY_FILE":               filepath.Join(baseTestPath, member, "peer", "tls.key"),
		"ETCD_LOG_OUTPUTS":                 "stdout",
		"ETCD_ENABLE_V2":                   "false",
		"ETCD_STRICT_RECONFIG_CHECK":       "true",
		"ETCD_DATA_DIR":                    "/data/test",
	}, c.Env)
	assert.Equal(t, "/path/etcd", c.EtcdBinaryFile)
	assert.Equal(t, "https://127.0.0.1:9080", c.LocalClientURL)
	assert.Equal(t, []string{
		"https://10.0.0.1:8080",
	}, c.InitialAdvertisePeerURLs)
	assert.Equal(t, []string{
		"https://10.0.0.1:8080",
		"https://10.0.0.2:8080",
	}, c.ClusterPeerURLs)
	assert.Equal(t, []string{
		"ETCD_CERT_FILE=" + filepath.Join(baseTestPath, member, "client", "tls.crt"),
		"ETCD_CLIENT_CERT_AUTH=true",
		"ETCD_DATA_DIR=/data/test",
		"ETCD_ENABLE_V2=false",
		"ETCD_INITIAL_ADVERTISE_PEER_URLS=https://10.0.0.1:8080",
		"ETCD_INITIAL_CLUSTER=node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080",
		"ETCD_KEY_FILE=" + filepath.Join(baseTestPath, member, "client", "tls.key"),
		"ETCD_LISTEN_CLIENT_URLS=https://10.1.0.1:9080,https://127.0.0.1:9080,https://10.0.0.1:9080",
		"ETCD_LOG_OUTPUTS=stdout",
		"ETCD_NAME=test",
		"ETCD_PEER_CERT_FILE=" + filepath.Join(baseTestPath, member, "peer", "tls.crt"),
		"ETCD_PEER_CLIENT_CERT_AUTH=true",
		"ETCD_PEER_KEY_FILE=" + filepath.Join(baseTestPath, member, "peer", "tls.key"),
		"ETCD_PEER_TRUSTED_CA_FILE=" + filepath.Join(baseTestPath, "peer", "ca.crt"),
		"ETCD_STRICT_RECONFIG_CHECK=true",
		"ETCD_TRUSTED_CA_FILE=" + filepath.Join(baseTestPath, "client", "ca.crt"),
	}, c.WriteEnv())
}
