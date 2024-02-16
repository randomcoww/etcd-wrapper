package main

import (
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/status"
)

func main() {
	args, err := arg.New()
	if err != nil {
		panic(err)
	}

	status := status.New(args)
	if err = status.Run(args); err != nil {
		panic(err)
	}
}
