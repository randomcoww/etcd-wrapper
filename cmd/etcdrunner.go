package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcd"
	r "github.com/randomcoww/etcd-wrapper/internal/runner"
	"github.com/randomcoww/etcd-wrapper/internal/s3util"
	s "github.com/randomcoww/etcd-wrapper/internal/snapshot"
	"go.uber.org/zap"
)

const (
	timeFormat      string = "20060102-150405"
	defaultRevision int64  = 1
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		os.Exit(1)
	}

	if err := run(logger); err != nil {
		logger.Error("runner", zap.Error(err))
		os.Exit(1)
	}
	os.Exit(0)
}

func run(logger *zap.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var (
		configPath = flag.String("config", "", "path to the YAML boot profile config file")
	)
	flag.Parse()

	cfg, err := c.LoadConfigFile(*configPath)
	if err != nil {
		return err
	}
	etcdConfig, err := c.LoadFromEnv()
	if err != nil {
		return err
	}
	s3Client, err := s3util.NewClient(cfg.S3Endpoint, cfg.S3Region, cfg.S3Bucket, cfg.S3AccessKeyID, cfg.S3SecretAccessKey, cfg.S3TLSConfig)
	if err != nil {
		return err
	}
	snapshot := s.NewFromConfig(logger, s3Client, cfg, func() string { return time.Now().UTC().Format(timeFormat) })
	runner := r.NewRunnerFromConfig(logger, &etcd.Fork{EtcdBinary: cfg.EtcdBinaryFile}, snapshot, cfg, etcdConfig)

	if _, err := runner.StartInit(ctx); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	}
}
