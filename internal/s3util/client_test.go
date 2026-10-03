// hits real local minio instance for test
// build with terraform/podman under test/

package s3util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/randomcoww/etcd-wrapper/config"
	"github.com/stretchr/testify/assert"
)

const (
	baseTestPath  string = "../../test/outputs"
	minioUser     string = "rootUser"
	minioPassword string = "rootPassword"
	minioBucket   string = "etcd"
)

func TestS3Client(t *testing.T) {
	yamlConfig := &config.YamlConfig{
		S3Endpoint:        "https://127.0.0.1:9000",
		S3Bucket:          minioBucket,
		S3Region:          "us-east-1",
		S3TrustedCAs:      []string{filepath.Join(baseTestPath, "minio", "certs", "CAs", "ca.crt")},
		S3AccessKeyID:     minioUser,
		S3SecretAccessKey: minioPassword,
	}

	client, err := NewClientFromConfig(yamlConfig)
	assert.NoError(t, err)

	clientCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// --- upload some test data ---

	for k, v := range map[string]string{
		"snap-1":  "snap-1-val",
		"snap-2":  "snap-2-val",
		"snap-3":  "",
		"other-1": "other-1-val",
	} {
		if _, err := uploadTestData(t, client.client, minioBucket, clientCtx, k, bytes.NewBufferString(v)); err != nil {
			t.Fatalf("Create test data: %v", err)
		}
	}

	ok, err := client.VerifyBucket(clientCtx)
	assert.NoError(t, err)
	assert.True(t, ok)

	list, errors := client.List(clientCtx, "snap-")
	assert.Equal(t, []string{"snap-1", "snap-2"}, list)
	assert.Equal(t, map[string]error{"snap-3": fmt.Errorf("object size is 0")}, errors)

	dataPath := t.TempDir()

	// --- download file with content ---

	snapshotFile, _ := os.CreateTemp(dataPath, "snapshot-restore-*.db")
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()

	ok, err = client.Download(clientCtx, "snap-2", func(ctx context.Context, reader io.Reader) error {
		b, err := io.Copy(snapshotFile, reader)
		if err != nil {
			return err
		}
		if b == 0 {
			return fmt.Errorf("snapshot file download size was 0")
		}
		return nil
	})
	assert.NoError(t, err) // no error
	assert.True(t, ok)     // true (exists)

	content, err := os.ReadFile(snapshotFile.Name())
	if err != nil {
		t.Fatal("test snapshot file missing: %w", err)
	}
	assert.Equal(t, "snap-2-val", string(content))

	// --- download zero length ---

	snapshotBad, _ := os.CreateTemp(dataPath, "snapshot-bad-*.db")
	defer os.RemoveAll(snapshotBad.Name())
	defer snapshotBad.Close()

	ok, err = client.Download(clientCtx, "snap-3", func(ctx context.Context, reader io.Reader) error {
		b, err := io.Copy(snapshotBad, reader)
		if err != nil {
			return err
		}
		if b == 0 {
			return fmt.Errorf("snapshot file download size was 0")
		}
		return nil
	})
	assert.Error(t, err) // picks up b == 0
	assert.True(t, ok)   // true (exists)

	// --- download non existent ---

	ok, err = client.Download(clientCtx, "non-existent", func(ctx context.Context, reader io.Reader) error {
		b, err := io.Copy(snapshotBad, reader)
		if err != nil {
			return err
		}
		if b == 0 {
			return fmt.Errorf("snapshot file download size was 0")
		}
		return nil
	})
	assert.NoError(t, err) // no error
	assert.False(t, ok)    // false (non-existent)
}

// --- helper ---

func uploadTestData(t *testing.T, client *minio.Client, bucket string, ctx context.Context, key string, reader io.Reader) (int64, error) {
	t.Helper()

	buf := &bytes.Buffer{}
	size, err := io.Copy(buf, reader)
	if err != nil {
		return size, fmt.Errorf("upload: failed to create buffer: %w", err)
	}
	if _, err = client.PutObject(ctx, bucket, key, buf, size, minio.PutObjectOptions{
		AutoChecksum: minio.ChecksumCRC32,
	}); err != nil {
		if cleanupErr := client.RemoveIncompleteUpload(ctx, bucket, key); cleanupErr != nil {
			return size, fmt.Errorf("upload: failed to put object: %w\n  failed to cleanup incomplete upload: %w", err, cleanupErr)
		}
		return size, fmt.Errorf("upload: failed to put object: %w", err)
	}
	return size, nil
}
