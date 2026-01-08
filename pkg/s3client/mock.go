package s3client

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"io"
	"os"
	"path/filepath"
)

const (
	baseTestPath    string = "../../test"
	minioBinaryFile string = "/minio/usr/bin/minio"
)

type mockClientSuccess struct {
	*client
}

type mockClientNoBackup struct {
	*client
}

func NewMockSuccessClient() *mockClientSuccess {
	return &mockClientSuccess{}
}

func NewMockNoBackupClient() *mockClientNoBackup {
	return &mockClientNoBackup{}
}

func (c *mockClientSuccess) Verify(ctx context.Context, config *c.Config) error {
	return nil
}

func (c *mockClientSuccess) Download(ctx context.Context, config *c.Config, handler func(context.Context, io.Reader) error) (bool, error) {
	file, err := os.Open(filepath.Join(baseTestPath, "test-snapshot.db"))
	if err != nil {
		return false, err
	}
	defer file.Close()
	return true, handler(ctx, file)
}

func (c *mockClientSuccess) Upload(ctx context.Context, config *c.Config, reader io.Reader) error {
	return nil
}

func (c *mockClientNoBackup) Verify(ctx context.Context, config *c.Config) error {
	return nil
}

func (c *mockClientNoBackup) Download(ctx context.Context, config *c.Config, handler func(context.Context, io.Reader) error) (bool, error) {
	return false, nil
}

func (c *mockClientNoBackup) Upload(ctx context.Context, config *c.Config, reader io.Reader) error {
	return nil
}
