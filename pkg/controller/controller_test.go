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
	dataPath, _ := os.MkdirTemp("", "data")
	defer os.RemoveAll(dataPath)

	configs := c.MockConfigs(dataPath)
	for _, config := range configs {
		p := etcdprocess.NewProcess(context.Background(), config)
		defer p.Wait()
		defer p.Stop()

		controller := &Controller{
			P:        p,
			S3Client: &s3client.MockClientNoBackup{},
		}
		err := controller.runEtcd(config)
		assert.NoError(t, err)

		time.Sleep(4 * time.Second)
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))
	client, err := etcdclient.NewClientFromPeers(ctx, configs[2])
	assert.NoError(t, err)

	_, err = client.Status(ctx, configs[2].LocalClientURL)
	assert.NoError(t, err)
}

func TestControllerRestore(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "data")
	defer os.RemoveAll(dataPath)

	configs := c.MockConfigs(dataPath)
	for _, config := range configs {
		p := etcdprocess.NewProcess(context.Background(), config)
		defer p.Wait()
		defer p.Stop()

		controller := &Controller{
			P:        p,
			S3Client: &s3client.MockClientSuccess{},
		}
		err := controller.runEtcd(config)
		assert.NoError(t, err)

		time.Sleep(4 * time.Second)
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))
	client, err := etcdclient.NewClientFromPeers(ctx, configs[2])
	assert.NoError(t, err)

	_, err = client.Status(ctx, configs[2].LocalClientURL)
	assert.NoError(t, err)
}
