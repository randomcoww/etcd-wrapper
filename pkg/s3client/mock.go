package s3client

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

type MockClientSuccess struct {
}

func (client *MockClientSuccess) Download(ctx context.Context, config *c.Config, handler func(context.Context, io.Reader) error) (bool, error) {
	file, err := os.Open(filepath.Join(baseTestPath, "test-snapshot.db"))
	if err != nil {
		return false, err
	}
	defer file.Close()
	return true, handler(ctx, file)
}

func (client *MockClientSuccess) Upload(ctx context.Context, config *c.Config, reader io.Reader) error {
	return nil
}

type MockClientNoBackup struct {
}

func (client *MockClientNoBackup) Download(ctx context.Context, config *c.Config, handler func(context.Context, io.Reader) error) (bool, error) {
	return false, nil
}

func (client *MockClientNoBackup) Upload(ctx context.Context, config *c.Config, reader io.Reader) error {
	return nil
}
