package s3util

import (
	"context"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
)

type Client struct {
	*minio.Client
}

func New(endpoint string) (*Client, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewEnvAWS(),
		Secure: true,
	})
	if err != nil {
		return nil, err
	}
	return &Client{
		minioClient,
	}, nil
}

func (v *Client) Download(ctx context.Context, bucket, key string, handler func(context.Context, io.Reader) error) (bool, error) {
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
		return true, err
	}
	return true, nil
}

func (v *Client) Upload(ctx context.Context, bucket, key string, r io.Reader) error {
	_, err := v.PutObject(ctx, bucket, key, r, -1, minio.PutObjectOptions{})
	return err
}
