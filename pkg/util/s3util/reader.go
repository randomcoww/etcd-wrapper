package s3util

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"io"
)

type reader struct {
	*s3.Client
}

type Reader interface {
	Open(context.Context, string) (io.ReadCloser, bool, error)
}

func NewReader(s3 *s3.Client) Reader {
	return &reader{s3}
}

// Open opens the file on path where path must be in the format "bucket/key"
func (v *reader) Open(ctx context.Context, path string) (io.ReadCloser, bool, error) {
	bucket, key, err := parseBucketAndKey(path)
	if err != nil {
		return nil, false, err
	}
	resp, err := v.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch apiError.(type) {
		case *types.NotFound, *types.NoSuchKey, *types.NoSuchBucket:
			return nil, false, nil
		}
		return nil, false, err
	}
	return resp.Body, true, nil
}
