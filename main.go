package main

import (
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/runner"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"os"
)

func main() {
	config, err := c.NewConfig(os.Args[1:])
	if err != nil {
		panic(err)
	}

	switch config.Cmd {
	case "run":
		p := etcdprocess.NewEtcdProcess()
		s3, err := s3client.NewClient(config)
		if err != nil {
			panic(err)
		}

		if err := runner.RunEtcd(config, p, s3); err != nil {
			panic(err)
		}

	case "backup":
		os.Exit(1)

	default:
		os.Exit(1)
	}
}
