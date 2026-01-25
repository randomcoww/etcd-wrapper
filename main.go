package main

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/runner"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func run() error {
	config, err := c.NewConfig(os.Args)
	if err != nil {
		config.Logger.Error("parse config", zap.Error(err))
		return err
	}

	p := etcdprocess.NewEtcdProcess()
	defer p.Wait()
	defer p.Stop()

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	s3, err := s3client.NewClient(config)
	if err != nil {
		config.Logger.Error("create s3 backup client", zap.Error(err))
		return err
	}
	verifyCtx, verifyCancel := context.WithTimeout(ctx, 4*time.Second)
	defer verifyCancel()

	err = s3.Verify(verifyCtx, config)
	if err != nil {
		config.Logger.Error("verify backup bucket", zap.Error(err))
		return err
	}

	if err := runner.RunEtcd(ctx, config, p, s3); err != nil {
		config.Logger.Error("start etcd", zap.Error(err))
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			timer := time.NewTimer(config.BackupInterval)
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				runner.RunBackup(ctx, config, s3)
			}
		}
	}()
	p.Wait()
	return nil
}

func main() {
	err := run()
	if err != nil {
		os.Exit(1)
	}
}
