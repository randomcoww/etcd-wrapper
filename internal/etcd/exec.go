package etcd

import (
	"syscall"
)

type Exec struct {
	EtcdBinary string
}

func (m *Exec) StartNew(env []string) error {
	return syscall.Exec(m.EtcdBinary,
		[]string{"--initial-cluster-state=new"},
		env,
	)
}

func (m *Exec) StartExisting(env []string) error {
	return syscall.Exec(m.EtcdBinary,
		[]string{"--initial-cluster-state=existing"},
		env,
	)
}

func (m *Exec) Stop() error {
	return nil
}

func (m *Exec) Wait() error {
	return nil
}
