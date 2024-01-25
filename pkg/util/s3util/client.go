package s3util

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"io"
)

type Client struct {
	*s3.Client
}

func New(s3 *s3.Client) *Client {
	return &Client{s3}
}

func (v *Client) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	bucket, key, err := parseBucketAndKey(path)
	if err != nil {
		return nil, err
	}
	resp, err := v.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *types.NotFound, *types.NoSuchKey, *types.NoSuchBucket:
				return nil, nil
			default:
				return nil, err
			}
		}
	}
	return resp.Body, nil
}

func (v *Client) Write(ctx context.Context, path string, r io.Reader) (int64, error) {
	bucket, key, err := parseBucketAndKey(path)
	if err != nil {
		return 0, err
	}
	rc := readCounter{Reader: r}
	_, err = manager.NewUploader(v.Client, func(u *manager.Uploader) {
		u.PartSize = uploadPartSize
	}).Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   rc,
	})
	if err != nil {
		return 0, err
	}
	return rc.contentLength, nil
}
