package controller

import (
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestRestoreSnapshot(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "data")
	defer os.RemoveAll(dataPath)

	config := memberConfigs(dataPath)[0]
	controller := &Controller{
		S3Client:       &s3client.MockClientSuccess{},
		restoreTimeout: 4 * time.Second,
	}

	ok, err := controller.restoreSnapshot(config)
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
		controller := &Controller{
			P:              etcdprocess.NewProcess(context.Background(), config),
			S3Client:       &s3client.MockClientNoBackup{},
			peerTimeout:    6 * time.Second,
			restoreTimeout: 4 * time.Second,
		}
		err := controller.runEtcd(config)
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
		controller := &Controller{
			P:              etcdprocess.NewProcess(context.Background(), config),
			S3Client:       &s3client.MockClientSuccess{},
			peerTimeout:    6 * time.Second,
			restoreTimeout: 4 * time.Second,
		}
		err := controller.runEtcd(config)
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
