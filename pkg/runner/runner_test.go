package runner

import (
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestRunnerFreshCluster(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s3 := &mockS3NoBackup{} // <-- simulate no backup found

	configs := mockConfigs(dataPath)
	for _, config := range configs {
		p := etcdprocess.NewEtcdProcess()
		defer p.Wait()
		defer p.Stop()

		err := RunEtcd(ctx, config, p, s3)
		assert.NoError(t, err)
		time.Sleep(config.ClusterTimeout + 2*time.Second)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		err := RunBackup(ctx, config, s3)
		assert.NoError(t, err)
	}
}

func TestRunnerWithRestore(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s3 := &mockS3{} // <-- simulate backup restored

	var ps []etcdprocess.EtcdProcess
	configs := mockConfigs(dataPath)
	for _, config := range configs {
		p := etcdprocess.NewEtcdProcess()
		defer p.Wait()
		defer p.Stop()
		ps = append(ps, p)

		err := RunEtcd(ctx, config, p, s3)
		assert.NoError(t, err)
		time.Sleep(config.ClusterTimeout + 2*time.Second)
	}

	for _, config := range configs {
		err := RunBackup(ctx, config, s3)
		assert.NoError(t, err)
	}

	// -- test replacing one node --- //

	for i := range configs[:1] {
		ps[i].Stop()
		ps[i].Wait()
	}

	for i, config := range configs[:1] {
		time.Sleep(config.ClusterTimeout + 2*time.Second)
		err := RunEtcd(ctx, config, ps[i], s3)
		assert.NoError(t, err)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		err := RunBackup(ctx, config, s3)
		assert.NoError(t, err)
	}

	// --- test replacing two nodes (break quorum) --- //

	for i := range configs[:2] {
		ps[i].Stop()
		ps[i].Wait()
	}

	for i, config := range configs[:2] {
		time.Sleep(config.ClusterTimeout + 2*time.Second)
		err := RunEtcd(ctx, config, ps[i], s3)
		assert.NoError(t, err)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		err := RunBackup(ctx, config, s3)
		assert.NoError(t, err)
	}

	// verify that test data is readable
	clientCtx, clientCancel := context.WithTimeout(ctx, time.Duration(20*time.Second))
	defer clientCancel()

	client, err := etcdclient.NewClientFromPeers(clientCtx, configs[2])
	assert.NoError(t, err)

	err = client.GetQuorum(clientCtx)
	assert.NoError(t, err)

	resp, err := client.C().KV.Get(clientCtx, "test-key1")
	assert.NoError(t, err)
	assert.Equal(t, "test-val1", string(resp.Kvs[0].Value)) // match data that should exist in the test data
}
