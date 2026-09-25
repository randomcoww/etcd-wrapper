package runner

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	c "github.com/randomcoww/etcd-wrapper/internal/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcd"
	"github.com/randomcoww/etcd-wrapper/internal/etcdclient"
	"github.com/stretchr/testify/assert"
)

// Fresh cluster with no existing data
// func TestNewWithNoDataCluster(t *testing.T) {
// 	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
// 	defer os.RemoveAll(dataPath)
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	configs, err := mockConfigs(dataPath)
// 	assert.NoError(t, err)

// 	for _, config := range configs {
// 		p := &etcd.Fork{Ctx: ctx}
// 		defer p.Wait()
// 		defer p.Stop()

// 		err := RunEtcd(ctx, config, p)
// 		assert.NoError(t, err)
// 		time.Sleep(config.InitialClusterTimeout + 2*time.Second)
// 	}

// 	// verify quorum, nodes, and backup
// 	for _, config := range configs {
// 		err := verifyTestStatus(ctx, config)
// 		assert.NoError(t, err)
// 	}
// }

// Restart cluster with different version data
// A separate controller ensures that the highest revision node starts first
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
			label: "mismatched revisions",
			snapshotRevFiles: []string{
				filepath.Join(baseTestPath, "../rev3-snap.db"),
				filepath.Join(baseTestPath, "../rev2-snap.db"),
				filepath.Join(baseTestPath, "../rev2-snap.db"),
			},
			queryKey:    "test-rev3",
			expectedVal: "test-rev3-val",
		},
		{
			label: "mismatched revisions",
			snapshotRevFiles: []string{
				filepath.Join(baseTestPath, "../rev3-snap.db"),
				"",
				"",
			},
			queryKey:    "test-rev3",
			expectedVal: "test-rev3-val",
		},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			dataPath := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			configs, err := mockConfigs(dataPath)
			assert.NoError(t, err)

			for i, config := range configs {
				p := &etcd.Fork{Ctx: ctx}
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

/*
func TestRunExistingCluster(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ps []*etcd.Fork
	configs, err := mockRunConfigs(dataPath)
	assert.NoError(t, err)

	for _, config := range configs {
		p := &etcd.Fork{Ctx: ctx}
		defer p.Wait()
		defer p.Stop()
		ps = append(ps, p)

		err := RunEtcd(ctx, config, p)
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
		err := RunEtcd(ctx, config, ps[i])
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
		err := RunEtcd(ctx, config, ps[i])
		assert.NoError(t, err)
	}

	// verify quorum, nodes, and backup
	for _, config := range configs {
		val, err := verifyTestData(ctx, config, "test-key1")
		assert.NoError(t, err)
		assert.Equal(t, "test-val1", val) // match value that should exist in the test data
	}
}
*/

func verifyTestStatus(ctx context.Context, config *c.Config) error {
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

func verifyTestData(ctx context.Context, config *c.Config, key string) (string, error) {
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
