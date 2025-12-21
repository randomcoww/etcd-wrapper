package main

import (
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/controller"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"os"
)

func main() {
	processCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := config.NewConfig(os.Args)
	if err != nil {
		panic(err)
	}
	etcdProcess := etcdprocess.NewProcess(processCtx, c)
	s3Client, err := s3client.NewClient(c)
	if err != nil {
		panic(err)
	}
	ctrl := &controller.Controller{
		P:        etcdProcess,
		S3Client: s3Client,
	}

	if err := ctrl.RunEtcd(c); err != nil {
		panic(err)
	}
	defer ctrl.P.Wait()
	defer ctrl.P.Stop()

	if err := ctrl.RunNode(processCtx, c); err != nil {
		panic(err)
	}
}
