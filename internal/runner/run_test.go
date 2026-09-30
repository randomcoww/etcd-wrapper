package runner

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/randomcoww/etcd-wrapper/clienttest"
	c "github.com/randomcoww/etcd-wrapper/internal/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcd"
	"github.com/randomcoww/etcd-wrapper/internal/etcdclient"
	"github.com/stretchr/testify/assert"
)

var (
	testSnapshot    string = filepath.Join(clienttest.BaseTestPath, "../rev3-snap.db")
	testSnapshotOld string = filepath.Join(clienttest.BaseTestPath, "../rev2-snap.db")
)

func TestCreateFromRestore(t *testing.T) {
	dataPath := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configs, err := clienttest.MockConfig(dataPath)
	if err != nil {
		t.Fatal("generate etcd mock configs: %w", err)
	}

	etcdPs, err := createTestEtcdCluster(t, ctx, configs, testSnapshot)
	if err != nil {
		t.Fatal("create test etcd cluster: %w", err)
	}
	for _, p := range etcdPs {
		defer p.Wait()
		defer p.Stop()
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		val, err := verifyTestData(t, ctx, config, "test-rev3")
		assert.NoError(t, err)
		assert.Equal(t, "test-rev3-val", val) // match value that should exist in the test data
	}
}

func TestReplaceOneMember(t *testing.T) {
	dataPath := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configs, err := clienttest.MockConfig(dataPath)
	if err != nil {
		t.Fatal("generate etcd mock configs: %w", err)
	}

	etcdPs, err := createTestEtcdCluster(t, ctx, configs, testSnapshot)
	if err != nil {
		t.Fatal("create test etcd cluster: %w", err)
	}
	for _, p := range etcdPs {
		defer p.Wait()
		defer p.Stop()
	}

	// stop one node
	for i, config := range configs[:1] {
		etcdPs[i].Stop()
		etcdPs[i].Wait()
		if err := clearExistingData(config); err != nil { // node restart and data loss
			t.Fatal("clear test data: %w", err)
		}
	}
	for i, config := range configs[:1] {
		if err := etcdclient.RestoreSnapshot(testSnapshotOld, config); err != nil { // restored old snapshot
			t.Fatal("add test snapshot: %w", err)
		}
		err := RunEtcd(ctx, config, etcdPs[i])
		assert.NoError(t, err)
		time.Sleep(config.InitialClusterTimeout + 2*time.Second)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		err := verifyTestStatus(t, ctx, config)
		assert.NoError(t, err)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		val, err := verifyTestData(t, ctx, config, "test-rev3")
		assert.NoError(t, err)
		assert.Equal(t, "test-rev3-val", val) // match value that should exist in the test data
	}
}

func TestReplaceTwoMembers(t *testing.T) {
	dataPath := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configs, err := clienttest.MockConfig(dataPath)
	if err != nil {
		t.Fatal("generate etcd mock configs: %w", err)
	}

	etcdPs, err := createTestEtcdCluster(t, ctx, configs, testSnapshot)
	if err != nil {
		t.Fatal("create test etcd cluster: %w", err)
	}
	for _, p := range etcdPs {
		defer p.Wait()
		defer p.Stop()
	}

	// stop two nodes
	for i, config := range configs[:2] {
		etcdPs[i].Stop()
		etcdPs[i].Wait()
		if err := clearExistingData(config); err != nil { // node restart and data loss
			t.Fatal("clear test data: %w", err)
		}
	}
	for i, config := range configs[:2] {
		if err := etcdclient.RestoreSnapshot(testSnapshotOld, config); err != nil { // restored old snapshot
			t.Fatal("add test snapshot: %w", err)
		}
		err := RunEtcd(ctx, config, etcdPs[i])
		assert.NoError(t, err)
		time.Sleep(config.InitialClusterTimeout + 2*time.Second)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		err := verifyTestStatus(t, ctx, config)
		assert.NoError(t, err)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		val, err := verifyTestData(t, ctx, config, "test-rev2") // reverts to older revision
		assert.NoError(t, err)
		assert.Equal(t, "test-rev2-val", val) // match value that should exist in the test data
	}
}

func verifyTestStatus(t *testing.T, ctx context.Context, config *c.Config) error {
	t.Helper()
	clientCtx, clientCancel := context.WithTimeout(ctx, time.Duration(config.ClientTimeout))
	defer clientCancel()

	client, err := etcdclient.NewClientFromPeers(clientCtx, config)
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

func verifyTestData(t *testing.T, ctx context.Context, config *c.Config, key string) (string, error) {
	t.Helper()
	clusterCtx, clusterCancel := context.WithTimeout(ctx, time.Duration(config.InitialClusterTimeout))
	defer clusterCancel()

	client, err := etcdclient.NewClientFromPeers(clusterCtx, config)
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

func createTestEtcdCluster(t *testing.T, ctx context.Context, configs []*c.Config, restoreFile string) ([]*etcd.Fork, error) {
	t.Helper()
	ps := make([]*etcd.Fork, len(configs))

	for i, config := range configs {
		p := &etcd.Fork{Ctx: ctx}
		ps[i] = p

		if err := etcdclient.RestoreSnapshot(restoreFile, config); err != nil {
			return nil, err
		}
		if err := RunEtcd(ctx, config, p); err != nil {
			return nil, err
		}
		time.Sleep(config.InitialClusterTimeout + 2*time.Second)
	}
	for _, config := range configs {
		if err := verifyTestStatus(t, ctx, config); err != nil {
			for _, p := range ps {
				defer p.Wait()
				defer p.Stop()
			}
			return nil, err
		}
	}
	return ps, nil
}
