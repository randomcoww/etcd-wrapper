package runner

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

const (
	baseTestPath string = "../../test"
)

func TestRunnerNew(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s3 := s3client.NewMockNoBackupClient() // <-- simulate no backup found

	configs := c.MockRunConfigs(dataPath)
	for _, config := range configs {
		p := etcdprocess.NewEtcdProcess()
		defer p.Wait()
		defer p.Stop()

		err := RunEtcd(ctx, config, p, s3)
		assert.NoError(t, err)
		time.Sleep(4 * time.Second)
	}

	for _, config := range configs {
		err := RunBackup(ctx, config, s3)
		assert.NoError(t, err)
	}
}

func TestRunnerRestore(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s3 := s3client.NewMockSuccessClient() // <-- simulate backup restored

	configs := c.MockRunConfigs(dataPath)
	for _, config := range configs {
		p := etcdprocess.NewEtcdProcess()
		defer p.Wait()
		defer p.Stop()

		err := RunEtcd(ctx, config, p, s3)
		assert.NoError(t, err)
		time.Sleep(4 * time.Second)
	}

	for _, config := range configs {
		err := RunBackup(ctx, config, s3)
		assert.NoError(t, err)
	}

	clientCtx, _ := context.WithTimeout(ctx, time.Duration(20*time.Second))
	client, err := etcdclient.NewClientFromPeers(clientCtx, configs[2])
	assert.NoError(t, err)

	resp, err := client.C().KV.Get(clientCtx, "test-key1")
	assert.NoError(t, err)
	assert.Equal(t, "test-val1", string(resp.Kvs[0].Value)) // match data that should exist in the test data
}
