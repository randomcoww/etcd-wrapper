package s3client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"go.uber.org/zap"
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
	Download(context.Context, *c.Config, string, func(context.Context, io.Reader) error) (bool, error)
	Upload(context.Context, *c.Config, string, io.Reader) error
	Remove(context.Context, *c.Config, []string) error
	List(context.Context, *c.Config) []string
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

func (c *client) Download(ctx context.Context, config *c.Config, key string, handler func(context.Context, io.Reader) error) (bool, error) {
	object, err := c.GetObject(ctx, config.S3BackupBucket, key, minio.GetObjectOptions{})
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

func (c *client) Upload(ctx context.Context, config *c.Config, key string, reader io.Reader) error {
	buf := &bytes.Buffer{}
	size, err := io.Copy(buf, reader)
	if err != nil {
		return fmt.Errorf("upload: failed to create buffer: %w", err)
	}
	if size == 0 {
		return fmt.Errorf("upload: size is 0")
	}
	if _, err = c.PutObject(ctx, config.S3BackupBucket, key, buf, size, minio.PutObjectOptions{
		AutoChecksum: minio.ChecksumCRC32,
	}); err != nil {
		if cleanupErr := c.cleanupIncomplete(config, key); cleanupErr != nil {
			return fmt.Errorf("upload: failed to put object: %w\n  failed to cleanup incomplete upload: %w", err, cleanupErr)
		}
		return fmt.Errorf("upload: failed to put object: %w", err)
	}
	return nil
}

func (c *client) cleanupIncomplete(config *c.Config, key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	return c.RemoveIncompleteUpload(ctx, config.S3BackupBucket, key)
}

func (c *client) Remove(ctx context.Context, config *c.Config, keys []string) error {
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for _, k := range keys {
			objectsCh <- minio.ObjectInfo{
				Key: k,
			}
		}
	}()

	var errs []error
	errorCh := c.RemoveObjects(ctx, config.S3BackupBucket, objectsCh, minio.RemoveObjectsOptions{})
	for e := range errorCh {
		errs = append(errs, e.Err)
	}
	return errors.Join(errs...)
}

func (c *client) List(ctx context.Context, config *c.Config) []string {
	objectCh := c.ListObjects(ctx, config.S3BackupBucket, minio.ListObjectsOptions{
		Prefix:    config.S3BackupKeyPrefix,
		Recursive: true,
	})
	var keys []string
	for object := range objectCh {
		if object.Err != nil {
			config.Logger.Error("list object error", zap.Error(object.Err))
			continue
		}
		if object.Size == 0 {
			config.Logger.Error("list object size was 0")
			continue
		}
		keys = append(keys, object.Key)
	}
	return keys
}
