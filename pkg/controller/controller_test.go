package controller

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"os"
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

func TestRestoreSnapshot(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "data")
	defer os.RemoveAll(dataPath)

	config := memberConfigs(dataPath)[0]
	controller := &Controller{
		S3Client:       &s3client.MockClientSuccess{},
		restoreTimeout: 4 * time.Second,
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(4*time.Second))
	ok, err := controller.restoreSnapshot(ctx, config)
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = etcdprocess.DataExists(config)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestRunEtcdNew(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "data")
	defer os.RemoveAll(dataPath)

	configs := memberConfigs(dataPath)
	for _, config := range configs {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(8*time.Second))
		controller := &Controller{
			P:              etcdprocess.NewProcess(context.Background(), config),
			S3Client:       &s3client.MockClientNoBackup{},
			peerTimeout:    6 * time.Second,
			clientTimeout:  4 * time.Second,
			restoreTimeout: 4 * time.Second,
		}
		err := controller.RunEtcd(ctx, config)
		assert.NoError(t, err)

		defer controller.P.Wait()
		defer controller.P.Stop()
	}

	clientCtx, _ := context.WithTimeout(context.Background(), time.Duration(30*time.Second))
	client, err := etcdclient.NewClientFromPeers(clientCtx, configs[0])
	assert.NoError(t, err)

	memberCtx, _ := context.WithTimeout(context.Background(), time.Duration(30*time.Second))
	list, err := client.MemberList(memberCtx)
	assert.NoError(t, err)
	assert.Equal(t, len(list.GetMembers()), len(configs))

	_, err = client.Status(memberCtx, configs[0].ListenClientURLs[0])
	assert.NoError(t, err)
}

func TestRunEtcdRestore(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "data")
	defer os.RemoveAll(dataPath)

	configs := memberConfigs(dataPath)
	for _, config := range configs {
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(8*time.Second))
		controller := &Controller{
			P:              etcdprocess.NewProcess(context.Background(), config),
			S3Client:       &s3client.MockClientSuccess{},
			peerTimeout:    6 * time.Second,
			clientTimeout:  4 * time.Second,
			restoreTimeout: 4 * time.Second,
		}
		err := controller.RunEtcd(ctx, config)
		assert.NoError(t, err)

		defer controller.P.Wait()
		defer controller.P.Stop()
	}

	clientCtx, _ := context.WithTimeout(context.Background(), time.Duration(30*time.Second))
	client, err := etcdclient.NewClientFromPeers(clientCtx, configs[0])
	assert.NoError(t, err)

	memberCtx, _ := context.WithTimeout(context.Background(), time.Duration(30*time.Second))
	list, err := client.MemberList(memberCtx)
	assert.NoError(t, err)
	assert.Equal(t, len(list.GetMembers()), len(configs))

	_, err = client.Status(memberCtx, configs[0].ListenClientURLs[0])
	assert.NoError(t, err)

	resp, err := client.C().KV.Get(memberCtx, "test-key1")
	assert.NoError(t, err)
	assert.Equal(t, "test-val1", string(resp.Kvs[0].Value)) // match data that should exist in snapshot
}

func memberConfigs(dataPath string) []*c.Config {
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
		config := &c.Config{
			EtcdBinaryFile:    etcdBinaryFile,
			EtcdutlBinaryFile: etcdutlBinaryFile,
			S3BackupEndpoint:  "https://test.internal",
			S3BackupBucket:    "bucket",
			S3BackupKey:       "path/key",
			Logger:            logger,
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
				"ETCD_INITIAL_CLUSTER_STATE":       "new",
			},
		}
		config.ParseEnvs()
		configs = append(configs, config)
	}
	return configs
}
