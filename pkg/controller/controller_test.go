package controller

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

func TestControllerNew(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	var controllers []*Controller
	configs := c.MockConfigs(dataPath)
	for _, config := range configs {
		p := etcdprocess.NewProcess(context.Background(), config)
		defer p.Wait()
		defer p.Stop()

		controllers = append(controllers, &Controller{
			P:        p,
			S3Client: &s3client.MockClientNoBackup{}, // <-- simulate no backup found
		})
	}

	for i, config := range configs {
		err := controllers[i].runEtcd(config)
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

func TestControllerRestore(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	var controllers []*Controller
	configs := c.MockConfigs(dataPath)
	for _, config := range configs {
		p := etcdprocess.NewProcess(context.Background(), config)
		defer p.Wait()
		defer p.Stop()

		controllers = append(controllers, &Controller{
			P:        p,
			S3Client: &s3client.MockClientSuccess{}, // <-- returns test backup data
		})
	}

	for i, config := range configs {
		err := controllers[i].runEtcd(config)
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
