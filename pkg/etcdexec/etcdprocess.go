package etcdexec

import (
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"syscall"
)

type EtcdExec struct {
}

func (m *EtcdExec) StartNew(config *c.Config) error {
	return syscall.Exec(config.EtcdBinaryFile,
		[]string{"--initial-cluster-state=new"},
		config.WriteEnv(),
	)
}

func (m *EtcdExec) StartExisting(config *c.Config) error {
	return syscall.Exec(config.EtcdBinaryFile,
		[]string{"--initial-cluster-state=existing"},
		config.WriteEnv(),
	)
}

func (m *EtcdExec) Stop() error {
	return nil
}

func (m *EtcdExec) Wait() error {
	return nil
}
