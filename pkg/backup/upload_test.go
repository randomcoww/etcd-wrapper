package backup

import (
	"bytes"
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestUploadSnapshot(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	config := mockConfig("upload", dataPath)
	t.Setenv("AWS_ACCESS_KEY_ID", minioUser)
	t.Setenv("AWS_SECRET_ACCESS_KEY", minioPassword)

	minioClient, err := s3client.NewClient(config)
	assert.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	err = minioClient.Verify(ctx, config)
	assert.NoError(t, err)

	// --- test data --- //

	baseNow, _ := time.Parse("2006-01-02", "2000-01-01")
	err = UploadSnapshot(ctx, config, minioClient, bytes.NewBufferString("test-data-1"),
		func() string { return baseNow.Add(time.Duration(1 * time.Minute)).Format(timeFormat) },
	)
	assert.NoError(t, err)

	err = UploadSnapshot(ctx, config, minioClient, bytes.NewBufferString("test-data-2"),
		func() string { return baseNow.Add(time.Duration(2 * time.Minute)).Format(timeFormat) },
	)
	assert.NoError(t, err)

	err = UploadSnapshot(ctx, config, minioClient, bytes.NewBufferString("test-data-3"),
		func() string { return baseNow.Add(time.Duration(3 * time.Minute)).Format(timeFormat) },
	)
	assert.NoError(t, err)

	// --- list --- //

	assert.Equal(t, []string{
		config.S3BackupKeyPrefix + "20000101-000200",
		config.S3BackupKeyPrefix + "20000101-000300",
	}, minioClient.List(ctx, config))

	// --- cleanup --- //

	err = minioClient.Remove(ctx, config, []string{
		config.S3BackupKeyPrefix + "20000101-000200",
		config.S3BackupKeyPrefix + "20000101-000300",
	})
	assert.NoError(t, err)
}
