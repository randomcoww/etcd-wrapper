package s3util

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
	Download(ctx context.Context, bucket, key string, handler func(context.Context, io.Reader) error) (bool, error)
	Upload(ctx context.Context, bucket, key string, r io.Reader) error
}

func New(endpoint string) (*client, error) {
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

func (v *client) Download(ctx context.Context, bucket, key string, handler func(context.Context, io.Reader) error) (bool, error) {
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
	if err := handler(ctx, object); err != nil {
		if err != nil {
			switch minio.ToErrorResponse(err).StatusCode {
			case 404:
				return false, nil
			default:
				return false, err
			}
		}
	}
	return true, nil
}

func (v *client) Upload(ctx context.Context, bucket, key string, r io.Reader) error {
	_, err := v.PutObject(ctx, bucket, key, r, -1, minio.PutObjectOptions{
		AutoChecksum: minio.ChecksumCRC32,
	})
	return err
}
