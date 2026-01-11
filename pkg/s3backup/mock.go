package s3backup

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"io"
	"os"
	"path/filepath"
)

const (
	baseTestPath string = "../../test"
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

func (c *mockClientSuccess) RestoreSnapshot(ctx context.Context, config *c.Config, versionBump uint64) (bool, error) {
	file, err := os.Open(filepath.Join(baseTestPath, "test-snapshot.db"))
	if err != nil {
		return false, err
	}
	if err := restoreV3Snapshot(ctx, config, file.Name(), versionBump); err != nil {
		return false, err
	}
	return true, nil
}

func (c *mockClientSuccess) UploadSnapshot(ctx context.Context, config *c.Config, reader io.Reader) error {
	return nil
}

func (c *mockClientNoBackup) Verify(ctx context.Context, config *c.Config) error {
	return nil
}

func (c *mockClientNoBackup) RestoreSnapshot(ctx context.Context, config *c.Config, versionBump uint64) (bool, error) {
	return false, nil
}

func (c *mockClientNoBackup) UploadSnapshot(ctx context.Context, config *c.Config, reader io.Reader) error {
	return nil
}
