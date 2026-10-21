package snapshot

import (
	"context"
	"crypto/rand"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/randomcoww/etcd-wrapper/internal/s3util"
	"go.uber.org/zap"
)

const (
	minioUser     string = "rootUser"
	minioPassword string = "rootPassword"
	minioBucket   string = "etcd"
)

func BenchmarkCreateBackup(b *testing.B) {
	s3Client, err := s3util.NewClient("https://127.0.0.1:9000", "us-east-1", minioBucket, minioUser, minioPassword, []string{filepath.Join(baseTestPath, "minio", "certs", "CAs", "ca.crt")})
	if err != nil {
		b.Fatal("crete s3client: %w", err)
	}

	logger, _ := zap.NewProduction()
	baseNow, _ := time.Parse("2006-01-02", "2000-01-01")
	snapshot := &Snapshot{
		logger:                 logger,
		s3Client:               s3Client,
		encryptionPassword:     testEncryptionKey,
		objectPrefix:           "snap-",
		objectTagFunc:          func() string { return baseNow.Add(time.Duration(1 * time.Minute)).Format(timeFormat) },
		s3ClientTimeout:        2 * time.Second,
		restoreSnapshotTimeout: 2 * time.Second,
		uploadSnapshotTimeout:  8 * time.Minute,
	}

	file := io.NopCloser(io.LimitReader(rand.Reader, 1*1024*1024*1024))
	clientCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := snapshot.CreateBackup(clientCtx, file); err != nil {
		b.Fatal("failed snapshot: %w", err)
	}

	if err := s3Client.Remove(clientCtx, []string{"snap-20000101-000100"}); err != nil {
		b.Fatal("failed cleanup: %w", err)
	}
}
