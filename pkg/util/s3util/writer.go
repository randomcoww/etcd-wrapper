package s3util

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/randomcoww/etcd-wrapper/pkg/backup/util"
	"io"
)

type writer struct {
	*s3.S3
}

type Writer interface {
	Write(context.Context, string, io.Reader) (int64, error)
	List(context.Context, string) ([]string, error)
	Delete(context.Context, string) error
}

func NewWriter(s3 *s3.S3) Writer {
	return &writer{s3}
}

// Write writes the backup file to the given s3 path, "bucket/key".
func (v *writer) Write(ctx context.Context, path string, r io.Reader) (int64, error) {
	buket, key, err := parseBucketAndKey(path)
	if err != nil {
		return 0, err
	}

	_, err = s3manager.NewUploaderWithClient(v.S3).UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   r,
	})
	if err != nil {
		return 0, err
	}

	resp, err := v.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.ContentLength == nil {
		return 0, fmt.Errorf("failed to compute s3 object size")
	}
	return *resp.ContentLength, nil
}

func (v *writer) List(ctx context.Context, basePath string) ([]string, error) {
	bucket, key, err := parseBucketAndKey(basePath)
	if err != nil {
		return nil, err
	}

	objects, err := v.ListObjectsWithContext(ctx, &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	objectKeys := []string{}
	for _, object := range objects.Contents {
		objectKeys = append(objectKeys, bucket+"/"+*object.Key)
	}
	return objectKeys, nil
}

func (v *writer) Delete(ctx context.Context, path string) error {
	bucket, key, err := parseBucketAndKey(path)
	if err != nil {
		return err
	}

	_, err = v.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}
