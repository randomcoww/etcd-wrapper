package runner

import (
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdfork"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestSnapshotBackup(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s3 := &mockS3NoBackup{} // <-- simulate no backup found

	configs, err := mockRunConfigs(dataPath)
	assert.NoError(t, err)

	// start etcd cluster to back up
	for _, config := range configs {
		p := &etcdfork.EtcdFork{Ctx: ctx}
		defer p.Wait()
		defer p.Stop()

		err := RunEtcd(ctx, config, p, s3)
		assert.NoError(t, err)
		time.Sleep(config.InitialClusterTimeout + 2*time.Second)
	}

	// -- test running backup -- //

	backupConfigs, err := mockBackupConfigs(dataPath)
	assert.NoError(t, err)

	// call backup from each member
	for _, config := range backupConfigs {
		err := RunBackup(ctx, config, s3)
		assert.NoError(t, err)
	}
}
