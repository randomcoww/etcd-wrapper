package reader

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/randomcoww/etcd-wrapper/pkg/backup/util"
	"io"
)

// ensure s3Reader satisfies reader interface.
var _ Reader = &s3Reader{}

// s3Reader provides Reader imlementation for reading a file from S3
type s3Reader struct {
	s3 *s3.S3
}

func NewS3Reader(s3 *s3.S3) Reader {
	return &s3Reader{s3}
}

// Open opens the file on path where path must be in the format "<s3-bucket-name>/<key>"
func (s3r *s3Reader) Open(path string) (io.ReadCloser, error) {
	bucket, key, err := util.ParseBucketAndKey(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse s3 bucket and key: %v", err)
	}
	resp, err := s3r.s3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}
