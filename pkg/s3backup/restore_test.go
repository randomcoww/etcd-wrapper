package s3backup

import (
	"bytes"
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestoreSnapshot(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	configs := c.MockRunConfigs(dataPath, "restore-test")
	config := configs[0]

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	minioClient := NewMockSuccessClient()
	ok, err := minioClient.RestoreSnapshot(ctx, config, 0)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestRestore(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	configs := c.MockRunConfigs(dataPath, "restore-test")
	config := configs[0]

	t.Setenv("AWS_ACCESS_KEY_ID", config.Env["AWS_ACCESS_KEY_ID"])
	t.Setenv("AWS_SECRET_ACCESS_KEY", config.Env["AWS_SECRET_ACCESS_KEY"])

	minioClient, err := NewClient(config)
	assert.NoError(t, err)

	clientCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	err = minioClient.Verify(clientCtx, config)
	assert.NoError(t, err)

	file, _ := os.Open(filepath.Join(baseTestPath, "test-snapshot.db"))
	defer file.Close()

	// --- test data --- //

	err = minioClient.upload(clientCtx, config, config.S3BackupKeyPrefix+"-1.db", file)
	assert.NoError(t, err)

	err = minioClient.upload(clientCtx, config, config.S3BackupKeyPrefix+"-2.db", bytes.NewBufferString("random-bad-data"))
	assert.NoError(t, err)

	// -- test restore -- //

	ok, err := minioClient.RestoreSnapshot(clientCtx, config, 10000)
	assert.NoError(t, err)
	assert.True(t, ok)

	// --- cleanup --- //

	err = minioClient.remove(clientCtx, config, []string{
		config.S3BackupKeyPrefix + "-1.db",
		config.S3BackupKeyPrefix + "-2.db",
	})
	assert.NoError(t, err)
}
