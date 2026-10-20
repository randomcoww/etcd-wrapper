package main

import (
	"context"
	"flag"
	"fmt"
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
	"go.uber.org/zap/zapcore"
)

const (
	timeFormat string = "20060102-150405"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Print(`Usage:

Start etcd:
etcd-wrapper init -config <yamlconfig>

Start periodic defragment and backup:
etcd-wrapper backup -config <yamlconfig>
`)
		os.Exit(0)
	}

	if err := run(os.Args[1], os.Args[2:]); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func run(cmd string, args []string) error {
	zapConfig := zap.NewProductionConfig()
	zapConfig.OutputPaths = []string{"stdout"}
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := zapConfig.Build()
	if err != nil {
		return err
	}
	defer logger.Sync()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	f := flag.NewFlagSet("config", flag.ExitOnError)
	configPath := f.String("config", "", "path to the YAML config file")
	f.Parse(args)

	cfg, err := c.LoadConfigFile(*configPath)
	if err != nil {
		logger.Error("load config", zap.Error(err))
		return err
	}
	etcdConfig, err := c.LoadFromEnv()
	if err != nil {
		logger.Error("load envs", zap.Error(err))
		return err
	}
	s3Client, err := s3util.NewClient(cfg.S3Endpoint, cfg.S3Region, cfg.S3Bucket, cfg.S3AccessKeyID, cfg.S3SecretAccessKey, cfg.S3TrustedCAs)
	if err != nil {
		logger.Error("create s3 client", zap.Error(err))
		return err
	}
	snapshot := s.NewFromConfig(logger, s3Client, cfg, func() string { return time.Now().UTC().Format(timeFormat) })

	switch cmd {
	case "init":
		etcdRunner, err := etcd.NewEtcdRunner(cfg.EtcdBinaryFile)
		if err != nil {
			logger.Error("etcd init", zap.Error(err))
			return err
		}
		defer etcdRunner.Stop()

		runner := r.NewRunnerFromConfig(logger, etcdRunner, snapshot, cfg, etcdConfig)
		if _, err := runner.StartInit(ctx); err != nil {
			logger.Error("etcd init run", zap.Error(err))
			return err
		}

	case "backup":
		backup := r.NewBackupFromConfig(logger, snapshot, cfg, etcdConfig)
		return backup.StartLoop(ctx)

	default:
		return fmt.Errorf("unsupported command")
	}

	select {
	case <-ctx.Done():
		if ctx.Err() != nil {
			logger.Error("finished", zap.Error(ctx.Err()))
			return ctx.Err()
		}
		logger.Info("finished")
		return nil
	}
}
