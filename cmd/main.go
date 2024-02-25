package main

import (
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/manifest"
	"github.com/randomcoww/etcd-wrapper/pkg/status"
)

func main() {
	args, err := arg.New()
	if err != nil {
		panic(err)
	}

	status := status.New(args, &manifest.EtcdPod{})
	if err = status.Run(args, 0); err != nil {
		panic(err)
	}
}
