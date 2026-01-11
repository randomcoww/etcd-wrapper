package s3backup

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

	configs := c.MockRunConfigs(dataPath, "test-client")
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

	// --- upload --- //

	err = minioClient.upload(clientCtx, config, config.S3BackupKeyPrefix+"-1.db", file)
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
		config.S3BackupKeyPrefix + "-1.db",
	}, keysFound)

	// --- download --- //

	dir, _ := os.MkdirTemp("", "etcd-wrapper-*")
	defer os.RemoveAll(dir)

	snapshotFile, _ := os.CreateTemp(dir, "snapshot-restore-*.db")
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()

	ok, err := minioClient.download(clientCtx, config, config.S3BackupKeyPrefix+"-1.db", func(ctx context.Context, reader io.Reader) error {
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

	// --- delete --- //

	err = minioClient.remove(clientCtx, config, []string{config.S3BackupKeyPrefix + "-1.db"})
	assert.NoError(t, err)

	// --- check deleted and no ley exists response --- //

	ok, err = minioClient.download(clientCtx, config, config.S3BackupKeyPrefix+"-1.db", func(ctx context.Context, reader io.Reader) error {
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
	assert.False(t, ok)
}
