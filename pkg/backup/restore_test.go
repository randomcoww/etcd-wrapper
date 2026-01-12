package backup

import (
	"bytes"
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestoreSnapshot(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	config := mockConfig("restore", dataPath)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	minioClient := &mockS3{}
	ok, err := RestoreSnapshot(ctx, config, minioClient, 0)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestRestore(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	config := mockConfig("restore", dataPath)
	t.Setenv("AWS_ACCESS_KEY_ID", minioUser)
	t.Setenv("AWS_SECRET_ACCESS_KEY", minioPassword)

	minioClient, err := s3client.NewClient(config)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	err = minioClient.Verify(ctx, config)
	assert.NoError(t, err)

	file, _ := os.Open(filepath.Join(baseTestPath, "../test-snapshot.db"))
	defer file.Close()

	// --- test data --- //

	err = minioClient.Upload(ctx, config, config.S3BackupKeyPrefix+"1.db", file)
	assert.NoError(t, err)

	err = minioClient.Upload(ctx, config, config.S3BackupKeyPrefix+"2.db", bytes.NewBufferString("random-bad-data"))
	assert.NoError(t, err)

	// -- test restore -- //

	ok, err := RestoreSnapshot(ctx, config, minioClient, 10000)
	assert.NoError(t, err)
	assert.True(t, ok)

	// --- cleanup --- //

	err = minioClient.Remove(ctx, config, []string{
		config.S3BackupKeyPrefix + "1.db",
		config.S3BackupKeyPrefix + "2.db",
	})
	assert.NoError(t, err)
}
