package s3util

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"io"
)

type writer struct {
	*s3.Client
}

type Writer interface {
	Write(context.Context, string, io.Reader) (int64, error)
	List(context.Context, string) ([]string, error)
	Delete(context.Context, string) error
}

func NewWriter(s3 *s3.Client) Writer {
	return &writer{s3}
}

// Write writes the backup file to the given s3 path, "bucket/key".
func (v *writer) Write(ctx context.Context, path string, r io.Reader) (int64, error) {
	bucket, key, err := parseBucketAndKey(path)
	if err != nil {
		return 0, err
	}

	var partMiBs int64 = 10
	_, err = manager.NewUploader(v.Client, func(u *manager.Uploader) {
		u.PartSize = partMiBs * 1024 * 1024
	}).Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   r,
	})
	if err != nil {
		return 0, err
	}

	resp, err := v.GetObject(ctx, &s3.GetObjectInput{
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

	objects, err := v.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
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

	_, err = v.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{Objects: []types.ObjectIdentifier{
			types.ObjectIdentifier{Key: aws.String(key)},
		}},
	})
	return err
}
