package config

import (
	"flag"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

const (
	baseTestPath string = "../../test"
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

	c, err := NewConfig()
	assert.NoError(t, err)

	flag.CommandLine.Set("etcd-binary-file", "/path/etcd")
	flag.CommandLine.Set("etcdutl-binary-file", "/path/etcdutl")

	assert.Equal(t, c.Env, map[string]string{
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
	},
	)
	assert.Equal(t, c.EtcdBinaryFile, "/path/etcd")
	assert.Equal(t, c.EtcdutlBinaryFile, "/path/etcdutl")
	assert.Equal(t, c.ListenClientURLs, []string{
		"https://node0-0:9080",
		"https://node0-1:9080",
	})
	assert.Equal(t, c.InitialAdvertisePeerURLs, []string{
		"https://node0-0:8080",
		"https://node0-1:8080",
	})
	assert.Equal(t, c.ClusterPeerURLs, []string{
		"https://node0-0:8080",
		"https://node1-0:8080",
	})
	assert.Equal(t, c.WriteEnv(), []string{
		"ETCDCTL_API=3",
		"ETCD_CERT_FILE=" + filepath.Join(baseTestPath, member, "client", "cert.pem"),
		"ETCD_CLIENT_CERT_AUTH=true",
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
	})
}
