package controller

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
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

func TestCreateNew(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(4*time.Second))
	clientCtx, _ := context.WithTimeout(context.Background(), time.Duration(30*time.Second))

	configs := memberConfigs()
	for _, config := range configs {
		controller, err := NewController(context.Background(), config)
		assert.NoError(t, err)

		err = controller.Run(ctx, config)
		assert.NoError(t, err)

		defer etcdprocess.RemoveDataDir(config)
		defer controller.P.Stop()
	}

	client, err := etcdclient.NewClientFromPeers(clientCtx, configs[0])
	assert.NoError(t, err)

	list, err := client.MemberList(clientCtx)
	assert.NoError(t, err)
	assert.Equal(t, len(list.GetMembers()), len(configs))
}

func TestReplaceExisting(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(8*time.Second))
	clientCtx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))

	// create new cluster
	var controllers []*Controller
	configs := memberConfigs()
	for _, config := range configs {
		controller, err := NewController(context.Background(), config)
		controllers = append(controllers, controller)
		assert.NoError(t, err)

		err = controller.Run(ctx, config)
		assert.NoError(t, err)

		defer etcdprocess.RemoveDataDir(config)
		defer controller.P.Stop()
	}

	// wait for cluster
	client, err := etcdclient.NewClientFromPeers(clientCtx, configs[0])
	assert.NoError(t, err)

	// test stopping node0
	err = controllers[0].P.Stop()
	assert.NoError(t, err)
	err = etcdprocess.RemoveDataDir(configs[0])
	assert.NoError(t, err)

	// run to replace node0
	err = controllers[0].Run(ctx, configs[0])
	assert.NoError(t, err)

	// check result
	client, err = etcdclient.NewClientFromPeers(clientCtx, configs[0])
	assert.NoError(t, err)

	list, err := client.MemberList(clientCtx)
	assert.NoError(t, err)
	assert.Equal(t, len(list.GetMembers()), len(configs))
}

func memberConfigs() []*c.Config {
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
				"ETCD_INITIAL_CLUSTER_TOKEN":       "test",
			},
		}
		config.ParseEnvs()
		configs = append(configs, config)
	}
	return configs
}
