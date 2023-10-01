package s3util

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"io"
)

type reader struct {
	*s3.S3
}

type Reader interface {
	Open(string) (io.ReadCloser, error)
}

func NewReader(s3 *s3.S3) Reader {
	return &reader{s3}
}

// Open opens the file on path where path must be in the format "bucket/key"
func (v *reader) Open(path string) (io.ReadCloser, error) {
	bucket, key, err := parseBucketAndKey(path)
	if err != nil {
		return nil, err
	}
	resp, err := v.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}
