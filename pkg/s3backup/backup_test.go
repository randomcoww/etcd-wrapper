package s3backup

import (
	"bytes"
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestUploadSnapshot(t *testing.T) {
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

	// --- test data --- //

	baseNow, _ := time.Parse("2006-01-02", "2000-01-01")

	minioClient.now = func() time.Time { return baseNow.Add(time.Duration(1 * time.Minute)) }
	err = minioClient.UploadSnapshot(clientCtx, config, bytes.NewBufferString("test-data-1"))
	assert.NoError(t, err)

	minioClient.now = func() time.Time { return baseNow.Add(time.Duration(2 * time.Minute)) }
	err = minioClient.UploadSnapshot(clientCtx, config, bytes.NewBufferString("test-data-2"))
	assert.NoError(t, err)

	minioClient.now = func() time.Time { return baseNow.Add(time.Duration(3 * time.Minute)) }
	err = minioClient.UploadSnapshot(clientCtx, config, bytes.NewBufferString("test-data-3"))
	assert.NoError(t, err)

	// --- list --- //

	var keysFound []string
	objects, err := minioClient.list(clientCtx, config)
	assert.NoError(t, err)
	for _, obj := range objects {
		assert.NoError(t, obj.Err)
		keysFound = append(keysFound, obj.Key)
	}
	assert.Equal(t, []string{
		"restore-test-20000101-000200",
		"restore-test-20000101-000300",
	}, keysFound)

	// --- cleanup --- //

	err = minioClient.remove(clientCtx, config, []string{
		"restore-test-20000101-000200",
		"restore-test-20000101-000300",
	})
	assert.NoError(t, err)
}
