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
)

const (
	timeFormat string = "20060102-150405"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		fmt.Print(`Usage:

Start etcd:
etcdwrapper init -config <yamlconfig>

Start periodic defragment and backup:
etcdwrapper backup -config <yamlconfig>
`)
		os.Exit(0)
	}

	if err := run(logger, os.Args[1], os.Args[2:]); err != nil {
		logger.Error("runner", zap.Error(err))
		os.Exit(1)
	}
	os.Exit(0)
}

func run(logger *zap.Logger, cmd string, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	f := flag.NewFlagSet("config", flag.ExitOnError)
	configPath := f.String("config", "", "path to the YAML config file")
	f.Parse(args)

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

	errCh := make(chan error)
	switch cmd {
	case "init":
		runner := r.NewRunnerFromConfig(logger, &etcd.Fork{EtcdBinary: cfg.EtcdBinaryFile}, snapshot, cfg, etcdConfig)
		if _, err := runner.StartInit(ctx); err != nil {
			return err
		}

	case "backup":
		backup := r.NewBackupFromConfig(logger, snapshot, cfg, etcdConfig)
		go func() {
			errCh <- backup.StartLoop(ctx)
		}()

	default:
		return fmt.Errorf("unsupported command")
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
