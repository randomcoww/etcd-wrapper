package runner

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdfork"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestRunNewCluster(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s3 := &mockS3NoBackup{} // <-- simulate no backup found

	configs, err := mockRunConfigs(dataPath)
	assert.NoError(t, err)

	for _, config := range configs {
		p := &etcdfork.EtcdFork{Ctx: ctx}
		defer p.Wait()
		defer p.Stop()

		err := RunEtcd(ctx, config, p, s3)
		assert.NoError(t, err)
		time.Sleep(config.InitialClusterTimeout + 2*time.Second)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		err := verifyTestStatus(ctx, config)
		assert.NoError(t, err)
	}
}

func TestRunExistingCluster(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s3 := &mockS3{} // <-- simulate backup restored

	var ps []*etcdfork.EtcdFork
	configs, err := mockRunConfigs(dataPath)
	assert.NoError(t, err)

	for _, config := range configs {
		p := &etcdfork.EtcdFork{Ctx: ctx}
		defer p.Wait()
		defer p.Stop()
		ps = append(ps, p)

		err := RunEtcd(ctx, config, p, s3)
		assert.NoError(t, err)
		time.Sleep(config.InitialClusterTimeout + 2*time.Second)
	}

	for _, config := range configs {
		err := verifyTestStatus(ctx, config)
		assert.NoError(t, err)
	}

	// -- test replacing one node --- //

	for i := range configs[:1] {
		ps[i].Stop()
		ps[i].Wait()
	}

	for i, config := range configs[:1] {
		time.Sleep(config.InitialClusterTimeout + 2*time.Second)
		err := RunEtcd(ctx, config, ps[i], s3)
		assert.NoError(t, err)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		err := verifyTestStatus(ctx, config)
		assert.NoError(t, err)
	}

	// --- test replacing two nodes (break quorum) --- //

	for i := range configs[:2] {
		ps[i].Stop()
		ps[i].Wait()
	}

	for i, config := range configs[:2] {
		time.Sleep(config.InitialClusterTimeout + 2*time.Second)
		err := RunEtcd(ctx, config, ps[i], s3)
		assert.NoError(t, err)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		val, err := verifyTestData(ctx, config, "test-key1")
		assert.NoError(t, err)
		assert.Equal(t, "test-val1", val) // match value that should exist in the test data
	}
}

func verifyTestStatus(ctx context.Context, config *c.Config) error {
	clientCtx, clientCancel := context.WithTimeout(ctx, time.Duration(config.ClientTimeout))
	defer clientCancel()

	client, err := etcdclient.NewClientFromPeersWithQuorum(clientCtx, config)
	if err != nil {
		return err
	}

	statusCtx, statusCancel := context.WithTimeout(ctx, time.Duration(config.ClientTimeout))
	defer statusCancel()
	if _, err := client.Status(statusCtx, config.LocalClientURL); err != nil {
		return err
	}
	return nil
}

func verifyTestData(ctx context.Context, config *c.Config, key string) (string, error) {
	clusterCtx, clusterCancel := context.WithTimeout(ctx, time.Duration(config.InitialClusterTimeout))
	defer clusterCancel()

	client, err := etcdclient.NewClientFromPeersWithQuorum(clusterCtx, config)
	if err != nil {
		return "", err
	}

	clientCtx, clientCancel := context.WithTimeout(ctx, 2*time.Second)
	defer clientCancel()
	resp, err := client.C().KV.Get(clientCtx, key)
	if err != nil {
		return "", err
	}
	return string(resp.Kvs[0].Value), nil
}
