package etcd

import (
	"syscall"

	c "github.com/randomcoww/etcd-wrapper/internal/config"
)

type Exec struct {
}

func (m *Exec) StartNew(config *c.Config) error {
	return syscall.Exec(config.EtcdBinaryFile,
		[]string{"--initial-cluster-state=new"},
		config.WriteEnv(),
	)
}

func (m *Exec) StartExisting(config *c.Config) error {
	return syscall.Exec(config.EtcdBinaryFile,
		[]string{"--initial-cluster-state=existing"},
		config.WriteEnv(),
	)
}

func (m *Exec) Stop() error {
	return nil
}

func (m *Exec) Wait() error {
	return nil
}
