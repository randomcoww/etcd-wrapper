package etcdprocess

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	clientPortBase    int    = 8080
	peerPortBase      int    = 8090
	etcdBinaryFile    string = "/mnt/usr/local/bin/etcd"
	etcdutlBinaryFile string = "/mnt/usr/local/bin/etcdutl"
	baseTestPath      string = "../../test"
)

func TestCreateNewCluster(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))

	configs := memberConfigs("new")
	var config *c.Config
	for _, config = range configs {
		p := NewProcess(context.Background(), config)
		err := p.Reconfigure(config)
		assert.NoError(t, err)

		defer RemoveDataDir(config)
		defer p.Stop()
	}

	client, err := etcdclient.NewClientFromPeers(ctx, config)
	assert.NoError(t, err)

	err = client.GetHealth(ctx)
	assert.NoError(t, err)

	list, err := client.MemberList(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(list.GetMembers()), len(configs))
}

func TestCreateClusterFromSnapshotRestore(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))

	configs := memberConfigs("existing")
	var config *c.Config
	for _, config = range configs {
		err := RestoreV3Snapshot(ctx, config, filepath.Join(baseTestPath, "snapshot.db"))
		assert.NoError(t, err)

		ok, err := DataExists(config)
		assert.NoError(t, err)
		assert.True(t, ok)

		p := NewProcess(context.Background(), config)
		err = p.Reconfigure(config)
		assert.NoError(t, err)

		defer RemoveDataDir(config)
		defer p.Stop()
	}

	client, err := etcdclient.NewClientFromPeers(ctx, config)
	assert.NoError(t, err)

	err = client.GetHealth(ctx)
	assert.NoError(t, err)

	list, err := client.MemberList(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(list.GetMembers()), len(configs))
}

func memberConfigs(state string) []*c.Config {
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

	var configs []*c.Config
	for i, member := range members {
		testDir := filepath.Join(baseTestPath, member+".etcd")

		config := &c.Config{
			EtcdBinaryFile:    etcdBinaryFile,
			EtcdutlBinaryFile: etcdutlBinaryFile,
			Logger:            logger,
			Env: map[string]string{
				"ETCD_DATA_DIR":                    testDir,
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
				"ETCD_INITIAL_CLUSTER_STATE":       state,
				"ETCD_INITIAL_CLUSTER_TOKEN":       "test",
			},
		}
		config.ParseEnvs()
		configs = append(configs, config)
	}
	return configs
}
