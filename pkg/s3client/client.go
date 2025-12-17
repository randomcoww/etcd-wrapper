package s3client

import (
	"bytes"
	"context"
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
	Download(context.Context, *c.Config, func(context.Context, io.Reader) (bool, error)) (bool, error)
	Upload(context.Context, *c.Config, io.Reader) (bool, error)
}

func NewClient(config *c.Config) (*client, error) {
	opts := &minio.Options{
		Creds:  credentials.NewEnvAWS(),
		Secure: true,
	}
	if config.S3TLSConfig != nil {
		opts.Transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second, // value taken from http.DefaultTransport
			}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second, // value taken from http.DefaultTransport
			TLSClientConfig:     config.S3TLSConfig,
		}
	}
	minioClient, err := minio.New(config.S3BackupEndpoint, opts)
	if err != nil {
		return nil, err
	}
	return &client{
		minioClient,
	}, nil
}

func (v *client) Download(ctx context.Context, config *c.Config, handler func(context.Context, io.Reader) (bool, error)) (bool, error) {
	object, err := v.GetObject(ctx, config.S3BackupBucket, config.S3BackupKey, minio.GetObjectOptions{})
	if err != nil {
		switch minio.ToErrorResponse(err).StatusCode {
		case 404:
			return false, nil
		default:
			return false, err
		}
	}
	defer object.Close()
	ok, err := handler(ctx, object)
	if err != nil {
		switch minio.ToErrorResponse(err).StatusCode {
		case 404:
			return false, nil
		default:
			return false, err
		}
	}
	if !ok {
		return false, nil
	}
	return true, nil
}

func (v *client) Upload(ctx context.Context, config *c.Config, reader io.Reader) (bool, error) {
	buf := &bytes.Buffer{}
	size, err := io.Copy(buf, reader)
	if err != nil {
		return false, err
	}
	if size == 0 {
		return false, nil
	}
	if _, err = v.PutObject(ctx, config.S3BackupBucket, config.S3BackupKey, buf, size, minio.PutObjectOptions{
		AutoChecksum: minio.ChecksumCRC32,
	}); err != nil {
		return false, err
	}
	return true, nil
}
