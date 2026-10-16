// hits real local minio instance for test
// build with terraform/podman under test/

package s3util

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/randomcoww/etcd-wrapper/internal/tlsutil"
	"github.com/stretchr/testify/assert"
)

const (
	baseTestPath  string = "../../test/outputs"
	minioUser     string = "rootUser"
	minioPassword string = "rootPassword"
	minioBucket   string = "etcd"
)

func TestS3Client(t *testing.T) {
	tlsConfig, err := tlsutil.BuildTLSCAConfig([]string{filepath.Join(baseTestPath, "minio", "certs", "CAs", "ca.crt")})
	if err != nil {
		t.Fatalf("TLS config: %v", err)
	}
	client, err := NewClient("127.0.0.1:9000", "us-east-1", minioBucket, minioUser, minioPassword, tlsConfig)
	assert.NoError(t, err)

	clientCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// --- upload test data ---

	err = client.Upload(clientCtx, "snap-1", bytes.NewBufferString("snap-1-val"))
	assert.NoError(t, err)

	err = client.Upload(clientCtx, "snap-2", bytes.NewBufferString("snap-2-val"))
	assert.NoError(t, err)

	err = client.Upload(clientCtx, "snap-3", bytes.NewBufferString(""))
	assert.NoError(t, err)

	err = client.Upload(clientCtx, "other-1", bytes.NewBufferString("other-1-val"))
	assert.NoError(t, err)

	// --- verify ---

	ok, err := client.VerifyBucket(clientCtx)
	assert.NoError(t, err)
	assert.True(t, ok)

	// --- list objects ---

	list, errors := client.List(clientCtx, "snap-")
	assert.Equal(t, []string{"snap-1", "snap-2"}, list)
	assert.Equal(t, map[string]error{"snap-3": fmt.Errorf("object size is 0")}, errors)

	// --- get objects ---

	reader, ok, err := client.Download(clientCtx, "snap-2")
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatal("snapshot copy: %w", err)
	}
	assert.Equal(t, "snap-2-val", buf.String())

	// --- remove objects and verify list ---

	err = client.Remove(clientCtx, []string{"snap-1", "snap-2", "snap-3"})
	assert.NoError(t, err)

	list, errors = client.List(clientCtx, "snap-")
	assert.Equal(t, []string{}, list)
	assert.Equal(t, map[string]error{}, errors)

	list, errors = client.List(clientCtx, "other-")
	assert.Equal(t, []string{"other-1"}, list)
	assert.Equal(t, map[string]error{}, errors)
}
