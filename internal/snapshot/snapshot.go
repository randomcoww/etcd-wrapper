package snapshot

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"filippo.io/age"
	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	"github.com/randomcoww/etcd-wrapper/internal/s3util"
	s "go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.uber.org/zap"
)

type Snapshot struct {
	logger                 *zap.Logger
	s3Client               s3util.Client
	encryptionPassword     string
	objectPrefix           string
	objectTagFunc          func() string
	retentionCount         int
	restoreRevisionBump    uint64
	s3ClientTimeout        time.Duration
	restoreSnapshotTimeout time.Duration
	uploadSnapshotTimeout  time.Duration
}

func NewFromConfig(logger *zap.Logger, s3Client s3util.Client, raw *c.YamlConfig, objectTagFunc func() string) *Snapshot {
	return &Snapshot{
		logger:                 logger,
		s3Client:               s3Client,
		encryptionPassword:     raw.SnapshotEncryptionKey,
		objectPrefix:           raw.SnapshotObjectPrefix,
		objectTagFunc:          objectTagFunc,
		retentionCount:         raw.SnapshotRetentionCount,
		restoreRevisionBump:    100000,
		s3ClientTimeout:        raw.S3ClientTimeout,
		restoreSnapshotTimeout: raw.RestoreSnapshotTimeout,
		uploadSnapshotTimeout:  raw.UploadSnapshotTimeout,
	}
}

func (s *Snapshot) VerifyBucket(ctx context.Context) (bool, error) {
	s3ClientCtx, cancel := context.WithTimeout(ctx, s.s3ClientTimeout)
	defer cancel()
	return s.s3Client.VerifyBucket(s3ClientCtx)
}

func (s *Snapshot) Restore(ctx context.Context, restoreConfig s.RestoreConfig) (bool, error) {
	ok, err := s.VerifyBucket(ctx)
	if err != nil {
		return false, fmt.Errorf("verify bucket access: %w", err)
	}
	if !ok {
		return false, nil
	}

	s3ClientCtx, cancel := context.WithTimeout(ctx, s.s3ClientTimeout)
	defer cancel()
	keys, errors := s.s3Client.List(s3ClientCtx, s.objectPrefix)
	if len(keys) == 0 {
		return false, nil
	}
	for object, err := range errors {
		s.logger.Error("list object error", zap.String("object", object), zap.Error(err))
	}

	for i := len(keys) - 1; i >= 0; i-- {
		ok, err = s.restoreKey(ctx, keys[i], restoreConfig)
		if err == nil && ok {
			s.logger.Info("restored from object", zap.String("key", keys[i]))
			break
		}
		s.logger.Error("restore from object", zap.String("key", keys[i]), zap.Error(err))
	}
	if err != nil {
		return false, fmt.Errorf("restore from object: %w", err)
	}
	return true, nil
}

func (s *Snapshot) restoreKey(ctx context.Context, key string, restoreConfig s.RestoreConfig) (bool, error) {
	s.logger.Info("attempting snapshot restore")
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
	s.logger.Info("opened file for snapshot")

	restoreSnapshotCtx, cancel := context.WithTimeout(ctx, s.restoreSnapshotTimeout)
	defer cancel()
	reader, ok, err := s.s3Client.Download(restoreSnapshotCtx, key)
	if err != nil {
		return false, fmt.Errorf("download snapshot: %w", err)
	}
	defer reader.Close()
	if !ok {
		s.logger.Info("no snapshots found")
		return false, nil
	}

	// --- assume age enryption, decrypt ---

	identity, err := age.NewScryptIdentity(s.encryptionPassword)
	if err != nil {
		return false, fmt.Errorf("decrypt identity: %w", err)
	}
	decryptedReader, err := age.Decrypt(reader, identity)
	if err != nil {
		return false, fmt.Errorf("decrypt reader: %w", err)
	}
	b, err := io.Copy(snapshotFile, decryptedReader)
	if err != nil {
		return false, fmt.Errorf("copy reader: %w", err)
	}
	if b == 0 {
		return false, fmt.Errorf("snapshot size is 0")
	}

	// --- restore to etcd ---

	restoreConfig.SnapshotPath = snapshotFile.Name()
	restoreConfig.RevisionBump = s.restoreRevisionBump
	if err := etcdutil.RestoreSnapshot(s.logger, restoreConfig); err != nil {
		return false, fmt.Errorf("restore snapshot to etcd: %w", err)
	}
	s.logger.Info("finished restoring snapshot")
	return true, nil
}

func (s *Snapshot) CreateBackup(ctx context.Context, reader io.ReadCloser) error {
	recipient, err := age.NewScryptRecipient(s.encryptionPassword)
	if err != nil {
		return fmt.Errorf("create scrypt recipient: %w", err)
	}

	pr, pw := io.Pipe()
	go func() {
		ageWriter, err := age.Encrypt(pw, recipient)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		_, copyErr := io.Copy(ageWriter, reader)
		ageCloseErr := ageWriter.Close()

		if copyErr != nil {
			pw.CloseWithError(copyErr)
		} else if ageCloseErr != nil {
			pw.CloseWithError(ageCloseErr)
		} else {
			pw.Close() // Finished successfully
		}
	}()

	// --- upload ---

	objectKey := fmt.Sprintf("%s%s", s.objectPrefix, s.objectTagFunc())
	s.logger.Info("encrypted backup object", zap.String("key", objectKey))

	uploadSnapshotCtx, uploadSnapshotCancel := context.WithTimeout(ctx, s.uploadSnapshotTimeout)
	defer uploadSnapshotCancel()
	if err := s.s3Client.Upload(uploadSnapshotCtx, objectKey, pr); err != nil {

		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, s.uploadSnapshotTimeout)
		defer cleanupCancel()
		if err := s.s3Client.RemoveIncompleteUpload(cleanupCtx, objectKey); err != nil {
			return fmt.Errorf("remove failed put: %w", err)
		}
		return fmt.Errorf("put object: %w", err)
	}
	s.logger.Info("created backup", zap.String("key", objectKey))
	return nil
}

func (s *Snapshot) RemoveOldBackups(ctx context.Context) error {
	s3ClientCtx, cancel := context.WithTimeout(ctx, s.s3ClientTimeout)
	defer cancel()
	keys, _ := s.s3Client.List(s3ClientCtx, s.objectPrefix)
	if len(keys) > s.retentionCount {
		removeKeys := keys[:len(keys)-s.retentionCount]
		if err := s.s3Client.Remove(s3ClientCtx, removeKeys); err != nil {
			return fmt.Errorf("remove old backups: %w", err)
		}
		s.logger.Info("removed old backups", zap.String("keys", fmt.Sprintf("%s", removeKeys)))
	}
	return nil
}
