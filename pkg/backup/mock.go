package backup

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/tlsutil"
	"go.uber.org/zap"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	minioUser     string = "rootUser"
	minioPassword string = "rootPassword"
	baseTestPath  string = "../../test/outputs"
	timeFormat    string = "20060102-150405"
)

type mockS3 struct{}

func (c *mockS3) Verify(ctx context.Context, config *c.Config) error {
	return nil
}

func (m *mockS3) Download(ctx context.Context, config *c.Config, key string, handler func(context.Context, io.Reader) error) (bool, error) {
	file, err := os.Open(filepath.Join(baseTestPath, "../test-snapshot.db"))
	if err != nil {
		return false, err
	}
	defer file.Close()
	return true, handler(ctx, file)
}

func (c *mockS3) Upload(ctx context.Context, config *c.Config, key string, reader io.Reader) error {
	return nil
}

func (c *mockS3) Remove(ctx context.Context, config *c.Config, keys []string) error {
	return nil
}

func (c *mockS3) List(ctx context.Context, config *c.Config) []string {
	return []string{
		"dummy", // just need non-zero keys
	}
}

func mockConfig(prefixKey, dataPath string) *c.Config {
	var (
		peerPortBase int    = 8090
		minioPort    int    = 9000
		member       string = "node0"
	)
	logger, _ := zap.NewProduction()

	config := &c.Config{
		EtcdutlBinaryFile: "/etcd/usr/local/bin/etcdutl",
		Logger:            logger,
		S3BackupHost:      fmt.Sprintf("127.0.0.1:%d", minioPort),
		S3BackupBucket:    "etcd",
		S3BackupKeyPrefix: fmt.Sprintf("%s-%d-", prefixKey, time.Now().Unix()),
		S3BackupCount:     2,
		RestoreTimeout:    2 * time.Second,
		UploadTimeout:     2 * time.Second,
		Env: map[string]string{
			"ETCD_NAME":                        member,
			"ETCD_INITIAL_CLUSTER":             fmt.Sprintf("%s=https://127.0.0.1:%d", member, peerPortBase),
			"ETCD_INITIAL_CLUSTER_TOKEN":       "test",
			"ETCD_INITIAL_ADVERTISE_PEER_URLS": fmt.Sprintf("https://127.0.0.1:%d", peerPortBase),
			"ETCD_DATA_DIR":                    dataPath,
		},
	}
	config.S3TLSConfig, _ = tlsutil.TLSCAConfig([]string{filepath.Join(baseTestPath, "minio", "certs", "CAs", "ca.crt")})
	config.WriteEnv()

	return config
}
