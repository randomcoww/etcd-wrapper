package s3client

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClient(t *testing.T) {
	dataPath, _ := os.MkdirTemp("", "etcd-test-*")
	defer os.RemoveAll(dataPath)

	configs := c.MockRunConfigs(dataPath)
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

	err = minioClient.Upload(clientCtx, config, file)
	assert.NoError(t, err)

	dir, _ := os.MkdirTemp("", "etcd-wrapper-*")
	defer os.RemoveAll(dir)

	snapshotFile, _ := os.CreateTemp(dir, "snapshot-restore-*.db")
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()

	ok, err := minioClient.Download(clientCtx, config, func(ctx context.Context, reader io.Reader) error {
		b, err := io.Copy(snapshotFile, reader)
		if err != nil {
			return err
		}
		if b == 0 {
			return fmt.Errorf("snapshot file download size was 0")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.True(t, ok)
}
