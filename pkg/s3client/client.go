package s3client

import (
	"bytes"
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"io"
	"net"
	"net/http"
	"time"
)

type client struct {
	*minio.Client
}

type Client interface {
	Verify(context.Context, *c.Config) error
	Download(context.Context, *c.Config, func(context.Context, io.Reader) error) (bool, error)
	Upload(context.Context, *c.Config, io.Reader) error
}

func NewClient(config *c.Config) (*client, error) {
	opts := &minio.Options{
		Creds:  credentials.NewEnvAWS(),
		Secure: true,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second, // value taken from http.DefaultTransport
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second, // value taken from http.DefaultTransport
			TLSClientConfig:     config.S3TLSConfig,
		},
	}
	minioClient, err := minio.New(config.S3BackupHost, opts)
	if err != nil {
		return nil, err
	}
	return &client{
		minioClient,
	}, nil
}

func (c *client) Verify(ctx context.Context, config *c.Config) error {
	ok, err := c.BucketExists(ctx, config.S3BackupBucket)
	if err != nil {
		return fmt.Errorf("failed to validate backup bucket: %w", err)
	}
	if !ok {
		return fmt.Errorf("backup bucket not found")
	}
	return nil
}

func (c *client) Download(ctx context.Context, config *c.Config, handler func(context.Context, io.Reader) error) (bool, error) {
	object, err := c.GetObject(ctx, config.S3BackupBucket, config.S3BackupKey, minio.GetObjectOptions{})
	if err != nil {
		return false, err
	}
	defer object.Close()
	_, err = object.Stat()
	if err != nil {
		switch minio.ToErrorResponse(err).Code {
		case minio.NoSuchKey, minio.NoSuchBucket:
			return false, nil
		default:
			return false, err
		}
	}
	return true, handler(ctx, object)
}

func (c *client) Upload(ctx context.Context, config *c.Config, reader io.Reader) error {
	buf := &bytes.Buffer{}
	size, err := io.Copy(buf, reader)
	if err != nil {
		return fmt.Errorf("upload: failed to create buffer: %w", err)
	}
	if size == 0 {
		return fmt.Errorf("upload: size is 0")
	}
	if _, err = c.PutObject(ctx, config.S3BackupBucket, config.S3BackupKey, buf, size, minio.PutObjectOptions{
		AutoChecksum: minio.ChecksumCRC32,
	}); err != nil {
		if cleanupErr := c.cleanupIncomplete(config); cleanupErr != nil {
			return fmt.Errorf("upload: failed to put object: %w\n  failed to cleanup incomplete upload: %w", err, cleanupErr)
		}
		return fmt.Errorf("upload: failed to put object: %w", err)
	}
	return nil
}

func (c *client) cleanupIncomplete(config *c.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	return c.RemoveIncompleteUpload(ctx, config.S3BackupBucket, config.S3BackupKey)
}
