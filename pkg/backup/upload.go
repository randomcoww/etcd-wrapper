package backup

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"go.uber.org/zap"
	"io"
)

func UploadSnapshot(ctx context.Context, config *c.Config, s3 s3client.Client, reader io.Reader, tagFunc func() string) error {
	if err := s3.Upload(ctx, config, fmt.Sprintf("%s%s", config.S3BackupKeyPrefix, tagFunc()), reader); err != nil {
		config.Logger.Error("upload backup failed", zap.Error(err))
		return err
	}

	keys := s3.List(ctx, config)
	if len(keys) > config.S3BackupCount {
		return s3.Remove(ctx, config, keys[:len(keys)-config.S3BackupCount])
	}
	return nil
}
