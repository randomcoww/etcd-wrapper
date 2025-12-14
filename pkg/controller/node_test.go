package controller

import (
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestNode(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "data")
	defer os.RemoveAll(dataPath)

	configs := memberConfigs(dataPath)
	var controllers []*Controller

	for _, config := range configs {
		controller := &Controller{
			P:              etcdprocess.NewProcess(context.Background(), config),
			S3Client:       &s3client.MockClientNoBackup{},
			peerTimeout:    6 * time.Second,
			restoreTimeout: 4 * time.Second,
			uploadTimeout:  4 * time.Second,
			statusTimeout:  4 * time.Second,
		}
		defer controller.P.Wait()
		defer controller.P.Stop()

		controllers = append(controllers, controller)
		err := controller.runEtcd(config)
		assert.NoError(t, err)
	}

	for i, config := range configs {
		err := controllers[i].runNode(config)
		assert.NoError(t, err)
	}
}
