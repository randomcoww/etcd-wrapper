package s3client

import (
	"context"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
)

type client struct {
	*minio.Client
}

type Client interface {
	Download(context.Context, string, string, func(context.Context, io.Reader) (bool, error)) (bool, error)
	Upload(context.Context, string, string, io.Reader, int64) error
}

func NewClient(endpoint string) (*client, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewEnvAWS(),
		Secure: true,
	})
	if err != nil {
		return nil, err
	}
	return &client{
		minioClient,
	}, nil
}

func (v *client) Download(ctx context.Context, bucket, key string, handler func(context.Context, io.Reader) (bool, error)) (bool, error) {
	object, err := v.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
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

func (v *client) Upload(ctx context.Context, bucket, key string, r io.Reader, fileSize int64) error {
	_, err := v.PutObject(ctx, bucket, key, r, fileSize, minio.PutObjectOptions{
		AutoChecksum: minio.ChecksumCRC32,
	})
	return err
}
