package config

import (
	"flag"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewConfig(t *testing.T) {
	t.Setenv("ETCD_LISTEN_CLIENT_URLS", "https://node0-0:9080,https://node0-1:9080")
	t.Setenv("ETCD_INITIAL_CLUSTER", "node0=https://node0-0:8080,node1=https://node1-0:8080")
	t.Setenv("ETCD_TRUSTED_CA_FILE", "test/ca-cert.pem")
	t.Setenv("ETCD_CERT_FILE", "test/client/cert.pem")
	t.Setenv("ETCD_KEY_FILE", "test/client/key.pem")
	t.Setenv("ETCD_PEER_TRUSTED_CA_FILE", "test/peer-ca-cert.pem")
	t.Setenv("ETCD_PEER_CERT_FILE", "test/peer/cert.pem")
	t.Setenv("ETCD_PEER_KEY_FILE", "test/peer/key.pem")

	c, err := NewConfig()
	assert.NoError(t, err)

	flag.CommandLine.Set("etcd-binary-file", "/path/etcd")
	flag.CommandLine.Set("etcdctl-binary-file", "/path/etcdctl")

	assert.Equal(t, c.Env, map[string]string{
		"ETCD_LISTEN_CLIENT_URLS":    "https://node0-0:9080,https://node0-1:9080,unixs://" + c.ClientSocketfile,
		"ETCD_INITIAL_CLUSTER":       "node0=https://node0-0:8080,node1=https://node1-0:8080",
		"ETCD_CLIENT_CERT_AUTH":      "true",
		"ETCD_TRUSTED_CA_FILE":       "test/ca-cert.pem",
		"ETCD_CERT_FILE":             "test/client/cert.pem",
		"ETCD_KEY_FILE":              "test/client/key.pem",
		"ETCD_PEER_CLIENT_CERT_AUTH": "true",
		"ETCD_PEER_TRUSTED_CA_FILE":  "test/peer-ca-cert.pem",
		"ETCD_PEER_CERT_FILE":        "test/peer/cert.pem",
		"ETCD_PEER_KEY_FILE":         "test/peer/key.pem",
		"ETCD_LOG_OUTPUTS":           "stdout",
		"ETCD_ENABLE_V2":             "false",
		"ETCD_STRICT_RECONFIG_CHECK": "true",
		"ETCDCTL_API":                "3",
	},
	)
	assert.Equal(t, c.EtcdBinaryFile, "/path/etcd")
	assert.Equal(t, c.EtcdctlBinaryFile, "/path/etcdctl")
	assert.Equal(t, c.ClusterPeerURLs, []string{
		"https://node0-0:8080",
		"https://node1-0:8080",
	})
}
