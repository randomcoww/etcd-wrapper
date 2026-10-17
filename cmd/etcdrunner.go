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
	g "github.com/randomcoww/etcd-wrapper/internal/gossip"
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
	if err := run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func run() error {
	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}

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
	etcdRunner := &etcd.Exec{
		EtcdBinary: cfg.EtcdBinaryFile,
	}
	snapshot := s.NewFromConfig(logger, s3Client, cfg, func() string { return time.Now().UTC().Format(timeFormat) })
	runner := r.NewRunnerFromConfig(logger, etcdRunner, snapshot, cfg, etcdConfig)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ok, err := runner.StartInit(ctx)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}

	errCh := make(chan error, 1)
	startCh := make(chan struct{})
	gossip, err := g.NewNode(logger, etcdConfig.Env["ETCD_NAME"], defaultRevision, cfg.GossipPeerURLs, etcdConfig.PeerTLSConfig, startCh)
	if err != nil {
		return err
	}
	defer gossip.Stop()

	go func() {
		errCh <- runner.StartLoop(ctx, startCh)
	}()
	go func() {
		errCh <- gossip.StartListen()
	}()
	go func() {
		errCh <- gossip.StartLoop()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case err := <-errCh:
		return err
	}
}
