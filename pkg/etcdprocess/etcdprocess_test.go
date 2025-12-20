package etcdprocess

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	baseTestPath string = "../../test"
)

func TestCreateNewCluster(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "data")
	defer os.RemoveAll(dataPath)

	configs := c.MockConfigs(dataPath)
	for _, config := range configs {
		p := NewProcess(context.Background(), config)
		err := p.StartNew()
		assert.NoError(t, err)

		defer p.Wait()
		defer p.Stop()
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))
	client, err := etcdclient.NewClientFromPeers(ctx, configs[0])
	assert.NoError(t, err)

	list, err := client.MemberList(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(configs), len(list.GetMembers()))

	_, err = client.Status(ctx, configs[0].LocalClientURL)
	assert.NoError(t, err)
}

func TestExistingFromSnapshotRestore(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "data")
	defer os.RemoveAll(dataPath)

	configs := c.MockConfigs(dataPath)
	for _, config := range configs[1:] { // recover 2 of 3 nodes
		restoreCtx, _ := context.WithTimeout(context.Background(), time.Duration(4*time.Second))
		err := RestoreV3Snapshot(restoreCtx, config, filepath.Join(baseTestPath, "test-snapshot.db"))
		assert.NoError(t, err)

		ok, err := DataExists(config)
		assert.NoError(t, err)
		assert.True(t, ok)
	}

	for _, config := range configs {
		p := NewProcess(context.Background(), config)
		err := p.StartExisting()
		assert.NoError(t, err)

		defer p.Wait()
		defer p.Stop()
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(20*time.Second))
	client, err := etcdclient.NewClientFromPeers(ctx, configs[0])
	assert.NoError(t, err)

	list, err := client.MemberList(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(configs), len(list.GetMembers()))

	_, err = client.Status(ctx, configs[0].LocalClientURL)
	assert.NoError(t, err)

	resp, err := client.C().KV.Get(ctx, "test-key1")
	assert.NoError(t, err)
	assert.Equal(t, "test-val1", string(resp.Kvs[0].Value)) // match data that should exist in snapshot
}
