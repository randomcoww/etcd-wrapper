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

	configs := c.MockRunConfigs(dataPath)
	for _, config := range configs {
		p := etcdprocess.NewMockEtcdProcess()
		defer p.Wait()
		defer p.Stop()
		s3 := s3client.NewMockNoBackupClient()

		err := RunEtcd(config, p, s3)
		assert.NoError(t, err)
		time.Sleep(4 * time.Second)
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))
	client, err := etcdclient.NewClientFromPeers(ctx, configs[2])
	assert.NoError(t, err)

	for _, config := range configs {
		_, err = client.Status(ctx, config.LocalClientURL)
		assert.NoError(t, err)
	}
}

func TestRunnerRestore(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	configs := c.MockRunConfigs(dataPath)
	for _, config := range configs {
		p := etcdprocess.NewMockEtcdProcess()
		defer p.Wait()
		defer p.Stop()
		s3 := s3client.NewMockSuccessClient()

		err := RunEtcd(config, p, s3)
		assert.NoError(t, err)
		time.Sleep(4 * time.Second)
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))
	client, err := etcdclient.NewClientFromPeers(ctx, configs[2])
	assert.NoError(t, err)

	for _, config := range configs {
		_, err = client.Status(ctx, config.LocalClientURL)
		assert.NoError(t, err)
	}

	resp, err := client.C().KV.Get(ctx, "test-key1")
	assert.NoError(t, err)
	assert.Equal(t, "test-val1", string(resp.Kvs[0].Value)) // match data that should exist in the test data
}
