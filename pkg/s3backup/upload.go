package s3backup

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"go.uber.org/zap"
	"io"
)

func (c *client) UploadSnapshot(ctx context.Context, config *c.Config, reader io.Reader) error {
	if err := c.upload(ctx, config, fmt.Sprintf("%s%s", config.S3BackupKeyPrefix, c.now().Format("20060102-150405")), reader); err != nil {
		config.Logger.Error("upload backup failed", zap.Error(err))
		return err
	}

	objects, err := c.list(ctx, config)
	if err != nil {
		config.Logger.Error("list backups failed", zap.Error(err))
		return err
	}

	var keys []string
	for i := 0; i < len(objects)-config.S3BackupCount; i++ {
		keys = append(keys, objects[i].Key)
	}

	return c.remove(ctx, config, keys)
}
