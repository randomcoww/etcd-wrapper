package main

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/runner"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"os"
	"time"
)

func main() {
	config, err := c.NewConfig(os.Args)
	if err != nil {
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := etcdprocess.NewEtcdProcess()
	defer p.Stop()

	s3, err := s3client.NewClient(config)
	if err != nil {
		os.Exit(1)
	}
	if err := runner.RunEtcd(ctx, config, p, s3); err != nil {
		os.Exit(1)
	}

	go func() {
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
