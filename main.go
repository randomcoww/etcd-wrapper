package main

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdexec"
	"github.com/randomcoww/etcd-wrapper/pkg/runner"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}

func run(args []string) error {
	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}

	config, err := c.NewConfig(args)
	if err != nil {
		logger.Error("parse args", zap.Error(err))
		return err
	}
	config.Logger = logger

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("Start etcd with", zap.Object("config", config))

	if err := runner.RunEtcd(ctx, config, &etcdexec.EtcdExec{}); err != nil {
		logger.Error("start etcd", zap.Error(err))
		return err
	}

	return nil
}
