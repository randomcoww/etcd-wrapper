package main

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdexec"
	"github.com/randomcoww/etcd-wrapper/pkg/runner"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) > 1 {
		if err := run(os.Args[1:]); err != nil {
			os.Exit(1)
		}
	}
	os.Exit(1)
}

func run(args []string) error {
	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}

	var cmd string
	if len(args) > 1 {
		cmd, args = args[0], args[1:]
	} else {
		logger.Error("not enough args provided")
		return fmt.Errorf("not enough arguments")
	}

	config, err := c.NewConfig(cmd, args)
	if err != nil {
		logger.Error("parse args", zap.Error(err))
		return err
	}
	config.Logger = logger

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	s3, err := s3client.NewClient(config)
	if err != nil {
		logger.Error("create s3 backup client", zap.Error(err))
		return err
	}

	switch cmd {
	case "run":
		logger.Info("Start etcd run with", zap.Object("config", config))

		if err := runner.RunEtcd(ctx, config, &etcdexec.EtcdExec{}, s3); err != nil {
			logger.Error("start etcd", zap.Error(err))
			return err
		}

	case "backup":
		logger.Info("start etcd backup with", zap.Object("config", config))

		verifyS3Ctx, verifyS3Cancel := context.WithTimeout(ctx, config.S3VerifyTimeout)
		defer verifyS3Cancel()

		err = s3.Verify(verifyS3Ctx, config)
		if err != nil {
			logger.Error("verify backup bucket", zap.Error(err))
			return err
		}

		for {
			timer := time.NewTimer(config.BackupInterval)
			select {
			case <-ctx.Done():
				return nil
			case <-timer.C:
				runner.RunBackup(ctx, config, s3)
			}
		}
	}
	return fmt.Errorf("unsupported command %s", cmd)
}
