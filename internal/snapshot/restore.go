package snapshot

import (
	"context"
	"fmt"
	"io"
	"os"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	"github.com/randomcoww/etcd-wrapper/internal/s3util"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.uber.org/zap"
)

type Restore struct {
	logger        *zap.Logger
	s3Client      s3util.Client
	restoreConfig snapshot.RestoreConfig
}

func NewRestoreFromConfig(logger *zap.Logger, env *c.EnvConfig, raw *c.YamlConfig) (*Restore, error) {
	s3Client, err := s3util.NewClientFromConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}
	return &Restore{
		logger:   logger,
		s3Client: s3Client,
		restoreConfig: snapshot.RestoreConfig{
			Name:                env.Env["ETCD_NAME"],
			OutputDataDir:       env.Env["ETCD_DATA_DIR"],
			OutputWALDir:        env.Env["ETCD_WAL_DIR"],
			PeerURLs:            env.InitialAdvertisePeerURLs,
			InitialCluster:      env.Env["ETCD_INITIAL_CLUSTER"],
			InitialClusterToken: env.Env["ETCD_INITIAL_CLUSTER_TOKEN"],
		},
	}, nil
}

func (r *Restore) VerifyBucket(ctx context.Context) (bool, error) {
	return r.s3Client.VerifyBucket(ctx)
}

func (r *Restore) Restore(ctx context.Context, objectPrefix string, revisionBump uint64) (bool, error) {
	ok, err := r.s3Client.VerifyBucket(ctx)
	if err != nil {
		return false, fmt.Errorf("verify bucket: %w", err)
	}
	if !ok {
		return false, nil
	}

	keys, errors := r.s3Client.List(ctx, objectPrefix)
	if len(keys) == 0 {
		return false, nil
	}
	for object, err := range errors {
		r.logger.Error("list", zap.String("object", object), zap.Error(err))
	}

	r.restoreConfig.RevisionBump = revisionBump
	for i := len(keys) - 1; i >= 0; i-- {
		ok, err = r.restoreKey(ctx, keys[i], revisionBump)
		if err == nil && ok {
			r.logger.Info("restored snapshot", zap.String("key", keys[i]))
			break
		}
		r.logger.Error("restore snapshot", zap.String("key", keys[i]), zap.Error(err))
	}
	if err != nil {
		return false, fmt.Errorf("restore snapshot: %w", err)
	}
	return true, nil
}

func (r *Restore) restoreKey(ctx context.Context, key string, revisionBump uint64) (bool, error) {
	r.logger.Info("attempting snapshot restore")
	dir, err := os.MkdirTemp("", "etcd-wrapper-*")
	if err != nil {
		return false, fmt.Errorf("create path for snapshot: %w", err)
	}
	defer os.RemoveAll(dir)

	snapshotFile, err := os.CreateTemp(dir, "snapshot-restore-*.db")
	if err != nil {
		return false, fmt.Errorf("open file for snapshot: %w", err)
	}
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()
	r.logger.Info("opened file for snapshot")

	ok, err := r.s3Client.Download(ctx, key, func(ctx context.Context, reader io.Reader) error {
		b, err := io.Copy(snapshotFile, reader)
		if err != nil {
			return err
		}
		if b == 0 {
			return fmt.Errorf("snapshot file download size was 0")
		}
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("download snapshot: %w", err)
	}
	if !ok {
		r.logger.Info("no snapshots found")
		return false, nil
	}

	r.restoreConfig.SnapshotPath = snapshotFile.Name()
	if err := etcdutil.RestoreSnapshot(r.logger, r.restoreConfig); err != nil {
		return false, fmt.Errorf("restore snapshot to etcd data: %w", err)
	}
	r.logger.Info("finished restoring snapshot")
	return true, nil
}
