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
)

func TestCreateCluster(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))

	withCluster(t, ctx, "new", func(client etcdclient.EtcdClient) {
		err := client.GetHealth(ctx)
		assert.NoError(t, err)
	})
}

func TestRestoreSnapshot(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))
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
	defer logger.Sync()

	var config *c.Config
	for i, member := range members {
		testDir := filepath.Join("test", member+".etcd")

		config = &c.Config{
			EtcdutlBinaryFile: etcdutlBinaryFile,
			Logger:            logger,
			Env: map[string]string{
				"ETCD_DATA_DIR":                    testDir,
				"ETCD_NAME":                        member,
				"ETCD_TRUSTED_CA_FILE":             filepath.Join("test", "ca-cert.pem"),
				"ETCD_CERT_FILE":                   filepath.Join("test", member, "client", "cert.pem"),
				"ETCD_KEY_FILE":                    filepath.Join("test", member, "client", "key.pem"),
				"ETCD_INITIAL_ADVERTISE_PEER_URLS": fmt.Sprintf("https://127.0.0.1:%d", peerPortBase+i),
				"ETCD_INITIAL_CLUSTER":             strings.Join(initialCluster, ","),
				"ETCD_INITIAL_CLUSTER_TOKEN":       "test",
			},
		}

		err := RestoreV3Snapshot(ctx, config, filepath.Join("test", "snapshot.db"))
		assert.NoError(t, err)

		ok, err := DataExists(config)
		assert.NoError(t, err)
		assert.True(t, ok)

		defer RemoveDataDir(config)
	}
}

func withCluster(t *testing.T, ctx context.Context, state string, handler func(etcdclient.EtcdClient)) {
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
	defer logger.Sync()

	var config *c.Config
	for i, member := range members {
		testDir := filepath.Join("test", member+".etcd")

		config = &c.Config{
			EtcdBinaryFile: "/mnt/usr/local/bin/etcd",
			Logger:         logger,
			Env: map[string]string{
				"ETCD_DATA_DIR":                    testDir,
				"ETCD_NAME":                        member,
				"ETCD_CLIENT_CERT_AUTH":            "true",
				"ETCD_PEER_CLIENT_CERT_AUTH":       "true",
				"ETCD_STRICT_RECONFIG_CHECK":       "true",
				"ETCD_TRUSTED_CA_FILE":             filepath.Join("test", "ca-cert.pem"),
				"ETCD_CERT_FILE":                   filepath.Join("test", member, "client", "cert.pem"),
				"ETCD_KEY_FILE":                    filepath.Join("test", member, "client", "key.pem"),
				"ETCD_PEER_TRUSTED_CA_FILE":        filepath.Join("test", "peer-ca-cert.pem"),
				"ETCD_PEER_CERT_FILE":              filepath.Join("test", member, "peer", "cert.pem"),
				"ETCD_PEER_KEY_FILE":               filepath.Join("test", member, "peer", "key.pem"),
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

		p := NewProcess(context.Background(), config)
		err := p.Run()
		assert.NoError(t, err)

		defer RemoveDataDir(config)
		defer p.Wait()
		defer p.Stop()
	}

	client, err := etcdclient.NewClientFromPeers(ctx, config)
	assert.NoError(t, err)
	handler(client)
}
