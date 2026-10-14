package s3util

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type s3client struct {
	client *minio.Client
	bucket string
}

type Client interface {
	VerifyBucket(context.Context) (bool, error)
	Download(context.Context, string) (io.ReadCloser, bool, error)
	List(context.Context, string) ([]string, map[string]error)
}

func NewClient(endpoint, region, bucket, accessKeyID, secretAccessKey string, tlsConfig *tls.Config) (*s3client, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: true,
		Region: region,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second, // value taken from http.DefaultTransport
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second, // value taken from http.DefaultTransport
			TLSClientConfig:     tlsConfig,
		},
	}
	client, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("creating S3 client: %w", err)
	}
	return &s3client{client: client, bucket: bucket}, nil
}

func (c *s3client) VerifyBucket(ctx context.Context) (bool, error) {
	return c.client.BucketExists(ctx, c.bucket)
}

func (c *s3client) Download(ctx context.Context, key string) (io.ReadCloser, bool, error) {
	object, err := c.client.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("get snapshot object: %w", err)
	}
	_, err = object.Stat()
	if err != nil {
		switch minio.ToErrorResponse(err).Code {
		case minio.NoSuchKey, minio.NoSuchBucket:
			return nil, false, nil
		default:
			return nil, false, fmt.Errorf("read snapshot object: %w", err)
		}
	}
	return object, true, nil
}

func (c *s3client) Upload(ctx context.Context, key string, reader io.ReadCloser) (int64, error) {
	defer reader.Close()

	buf := &bytes.Buffer{}
	size, err := io.Copy(buf, reader)
	if err != nil {
		return size, fmt.Errorf("put buffer: %w", err)
	}
	if _, err := c.client.PutObject(ctx, c.bucket, key, buf, size, minio.PutObjectOptions{
		AutoChecksum: minio.ChecksumCRC32,
	}); err != nil {
		removeIncompleteErr := c.client.RemoveIncompleteUpload(ctx, c.bucket, key)
		if removeIncompleteErr != nil {
			return size, fmt.Errorf("put object: %w", errors.Join(err, removeIncompleteErr))
		}
		return size, fmt.Errorf("put object: %w", err)
	}
	return size, nil
}

func (c *s3client) Remove(ctx context.Context, keys []string) error {
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
	errorCh := c.client.RemoveObjects(ctx, c.bucket, objectsCh, minio.RemoveObjectsOptions{})
	for e := range errorCh {
		errs = append(errs, e.Err)
	}
	return errors.Join(errs...)
}

func (c *s3client) List(ctx context.Context, prefix string) ([]string, map[string]error) {
	objectCh := c.client.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	keys := []string{}
	errors := make(map[string]error)

	for object := range objectCh {
		if object.Err != nil {
			errors[object.Key] = object.Err
			continue
		}
		if object.Size == 0 {
			errors[object.Key] = fmt.Errorf("object size is 0")
			continue
		}
		keys = append(keys, object.Key)
	}
	return keys, errors
}
