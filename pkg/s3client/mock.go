package s3client

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

const (
	baseTestPath string = "../../test"
)

type MockClientSuccess struct {
}

func (client *MockClientSuccess) Download(ctx context.Context, bucket, key string, handler func(context.Context, io.Reader) (bool, error)) (bool, error) {
	file, err := os.Open(filepath.Join(baseTestPath, "test-snapshot.db"))
	if err != nil {
		return false, err
	}
	defer file.Close()
	return handler(ctx, file)
}

func (client *MockClientSuccess) Upload(ctx context.Context, bucket, key string, r io.Reader, fileSize int64) error {
	return nil
}

type MockClientNoBackup struct {
}

func (client *MockClientNoBackup) Download(ctx context.Context, bucket, key string, handler func(context.Context, io.Reader) (bool, error)) (bool, error) {
	return false, nil
}

func (client *MockClientNoBackup) Upload(ctx context.Context, bucket, key string, r io.Reader, fileSize int64) error {
	return nil
}
