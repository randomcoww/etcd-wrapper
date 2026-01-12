package s3client

import (
	"bytes"
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/tlsutil"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	baseTestPath  string = "../../test/outputs"
	minioUser     string = "rootUser"
	minioPassword string = "rootPassword"
)

func TestClient(t *testing.T) {
	config := &c.Config{
		S3BackupHost:      "127.0.0.1:9000",
		S3BackupBucket:    "etcd",
		S3BackupKeyPrefix: fmt.Sprintf("client-%d-", time.Now().Unix()),
	}
	config.S3TLSConfig, _ = tlsutil.TLSCAConfig([]string{filepath.Join(baseTestPath, "minio", "certs", "CAs", "ca.crt")})
	t.Setenv("AWS_ACCESS_KEY_ID", minioUser)
	t.Setenv("AWS_SECRET_ACCESS_KEY", minioPassword)

	minioClient, err := NewClient(config)
	assert.NoError(t, err)

	clientCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	err = minioClient.Verify(clientCtx, config)
	assert.NoError(t, err)

	// --- upload --- //

	err = minioClient.Upload(clientCtx, config, config.S3BackupKeyPrefix+"1.db", bytes.NewBufferString("test-data-1"))
	assert.NoError(t, err)

	err = minioClient.Upload(clientCtx, config, config.S3BackupKeyPrefix+"2.db", bytes.NewBufferString("test-data-2"))
	assert.NoError(t, err)

	err = minioClient.Upload(clientCtx, config, config.S3BackupKeyPrefix+"3.db", bytes.NewBufferString(""))
	assert.Error(t, err)

	// --- list --- //

	keys := minioClient.List(clientCtx, config)
	assert.Equal(t, []string{
		config.S3BackupKeyPrefix + "1.db",
		config.S3BackupKeyPrefix + "2.db",
	}, keys)

	// --- download --- //

	dir, _ := os.MkdirTemp("", "etcd-wrapper-*")
	defer os.RemoveAll(dir)

	snapshotFile, _ := os.CreateTemp(dir, "snapshot-restore-*.db")
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()

	ok, err := minioClient.Download(clientCtx, config, config.S3BackupKeyPrefix+"2.db", func(ctx context.Context, reader io.Reader) error {
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

	err = minioClient.Remove(clientCtx, config, []string{
		config.S3BackupKeyPrefix + "1.db",
		config.S3BackupKeyPrefix + "2.db",
	})
	assert.NoError(t, err)

	// --- list empty --- //

	keys = minioClient.List(clientCtx, config)
	assert.Equal(t, 0, len(keys))

	// --- check deleted and no key exists response --- //

	ok, err = minioClient.Download(clientCtx, config, config.S3BackupKeyPrefix+"1.db", func(ctx context.Context, reader io.Reader) error {
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
