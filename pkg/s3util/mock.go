package s3util

import (
	"context"
	"io"
)

type MockClient struct {
	DownloadExists bool
	DownloadErr    error
	UploadErr      error
}

func (client *MockClient) Download(ctx context.Context, bucket, key string, handler func(context.Context, io.Reader) error) (bool, error) {
	return client.DownloadExists, client.DownloadErr
}

func (client *MockClient) Upload(ctx context.Context, bucket, key string, r io.Reader) error {
	return client.UploadErr
}
