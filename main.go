package main

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/runner"
	"github.com/randomcoww/etcd-wrapper/pkg/s3backup"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	config, err := c.NewConfig(os.Args)
	if err != nil {
		config.Logger.Error("parse config", zap.Error(err))
		os.Exit(1)
	}

	p := etcdprocess.NewEtcdProcess()
	defer p.Wait()
	defer p.Stop()

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	s3, err := s3backup.NewClient(config)
	if err != nil {
		config.Logger.Error("create s3 backup client", zap.Error(err))
		os.Exit(1)
	}
	verifyCtx, _ := context.WithTimeout(ctx, 4*time.Second)
	err = s3.Verify(verifyCtx, config)
	if err != nil {
		config.Logger.Error("verify backup bucket", zap.Error(err))
		os.Exit(1)
	}

	if err := runner.RunEtcd(ctx, config, p, s3); err != nil {
		config.Logger.Error("start etcd", zap.Error(err))
		os.Exit(1)
	}

	go func() {
		wg.Add(1)
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
}
