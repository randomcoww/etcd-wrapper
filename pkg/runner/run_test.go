package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdfork"
	"github.com/stretchr/testify/assert"
)

// Fresh cluster with no existing data
func TestNewWithNoDataCluster(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configs, err := mockConfigs(dataPath)
	assert.NoError(t, err)

	for _, config := range configs {
		p := &etcdfork.EtcdFork{Ctx: ctx}
		defer p.Wait()
		defer p.Stop()

		err := RunEtcd(ctx, config, p)
		assert.NoError(t, err)
		time.Sleep(config.InitialClusterTimeout + 2*time.Second)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		err := verifyTestStatus(ctx, config)
		assert.NoError(t, err)
	}
}

// Restart cluster with different version data
func TestStartWithExistingData(t *testing.T) {
	tests := []struct {
		label            string
		snapshotRevFiles []string
		queryKey         string
		expectedVal      string
	}{
		{
			label: "same revision",
			snapshotRevFiles: []string{
				filepath.Join(baseTestPath, "../rev3-snap.db"),
				filepath.Join(baseTestPath, "../rev3-snap.db"),
				filepath.Join(baseTestPath, "../rev3-snap.db"),
			},
			queryKey:    "test-rev3",
			expectedVal: "test-rev3-val",
		},
		{
			label: "one node data missing",
			snapshotRevFiles: []string{
				filepath.Join(baseTestPath, "../rev3-snap.db"),
				"",
				filepath.Join(baseTestPath, "../rev3-snap.db"),
			},
			queryKey:    "test-rev3",
			expectedVal: "test-rev3-val",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {

			dataPath, _ := os.MkdirTemp("", "etcd-test-*")
			defer os.RemoveAll(dataPath)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			configs, err := mockConfigs(dataPath)
			assert.NoError(t, err)

			for i, config := range configs {
				p := &etcdfork.EtcdFork{Ctx: ctx}
				defer p.Wait()
				defer p.Stop()

				// restore sample data
				if tt.snapshotRevFiles[i] != "" {
					err := etcdclient.RestoreSnapshot(tt.snapshotRevFiles[i], config)
					assert.NoError(t, err)
				}

				err = RunEtcd(ctx, config, p)
				assert.NoError(t, err)
				time.Sleep(config.InitialClusterTimeout + 2*time.Second)
			}

			// verify quorum, nodes, and backup
			for _, config := range configs {
				err := verifyTestStatus(ctx, config)
				assert.NoError(t, err)
			}

			// verify quorum, nodes, and backup
			for _, config := range configs {
				val, err := verifyTestData(ctx, config, tt.queryKey)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedVal, val) // match value that should exist in the test data
			}
		})
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
