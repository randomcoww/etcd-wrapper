package s3util

import (
	"context"
	"crypto/tls"
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

type S3Client interface {
	Verify(context.Context) error
	Download(context.Context, string, func(context.Context, io.Reader) error) (bool, error)
	List(context.Context, string) []string
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
	client, err := NewClient(u.Host, raw.S3Region, raw.S3Bucket, raw.S3AccessKeyID, raw.S3SecretAccessKey, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("client: %v", err)
	}
	return client, nil
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

func (c *s3client) Verify(ctx context.Context) error {
	ok, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("failed to validate backup bucket: %w", err)
	}
	if !ok {
		return fmt.Errorf("backup bucket not found")
	}
	return nil
}

func (c *s3client) Download(ctx context.Context, key string, handler func(context.Context, io.Reader) error) (bool, error) {
	object, err := c.client.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
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

func (c *s3client) List(ctx context.Context, prefix string) []string {
	objectCh := c.client.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	var keys []string
	for object := range objectCh {
		if object.Err != nil {
			continue
		}
		if object.Size == 0 {
			continue
		}
		keys = append(keys, object.Key)
	}
	return keys
}
