package s3util

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/tlsutil"
)

type s3client struct {
	client *minio.Client
	bucket string
}

type Client interface {
	VerifyBucket(context.Context) (bool, error)
	Download(context.Context, string, func(context.Context, io.ReadCloser) error) (bool, error)
	List(context.Context, string) ([]string, map[string]error)
}

func NewClientFromConfig(raw *c.YamlConfig) (*s3client, error) {
	if raw.S3Bucket == "" {
		return nil, fmt.Errorf("missing s3Bucket")
	}
	u, err := url.Parse(raw.S3Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse advertise url: %w", err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("S3 URL scheme must be HTTPS")
	}
	tlsConfig, err := tlsutil.BuildTLSCAConfig(raw.S3TrustedCAs)
	if err != nil {
		return nil, fmt.Errorf("building S3 TLS config: %w", err)
	}

	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(raw.S3AccessKeyID, raw.S3SecretAccessKey, ""),
		Secure: true,
		Region: raw.S3Region,
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
	client, err := minio.New(u.Host, opts)
	if err != nil {
		return nil, fmt.Errorf("creating S3 client: %w", err)
	}
	return &s3client{client: client, bucket: raw.S3Bucket}, nil
}

func (c *s3client) VerifyBucket(ctx context.Context) (bool, error) {
	return c.client.BucketExists(ctx, c.bucket)
}

func (c *s3client) Download(ctx context.Context, key string, handler func(context.Context, io.ReadCloser) error) (bool, error) {
	object, err := c.client.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return false, fmt.Errorf("get snapshot object: %w", err)
	}
	defer object.Close()
	_, err = object.Stat()
	if err != nil {
		switch minio.ToErrorResponse(err).Code {
		case minio.NoSuchKey, minio.NoSuchBucket:
			return false, nil
		default:
			return false, fmt.Errorf("read snapshot object: %w", err)
		}
	}
	return true, handler(ctx, object)
}

func (c *s3client) List(ctx context.Context, prefix string) ([]string, map[string]error) {
	objectCh := c.client.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	var keys []string
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
