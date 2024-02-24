package main

import (
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/status"
	"log"
)

func main() {
	args, err := arg.New()
	if err != nil {
		panic(err)
	}

	status := status.New(args)
	if err = status.Run(args, 0); err != nil {
		log.Printf("main exit: %v", err)
		panic(err)
	}
}
