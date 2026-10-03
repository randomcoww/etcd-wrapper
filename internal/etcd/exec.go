package etcd

import (
	"syscall"
)

type Exec struct {
	EtcdBinary string
}

func (m *Exec) StartNew(env []string) error {
	return m.start(env, "new")
}

func (m *Exec) StartExisting(env []string) error {
	return m.start(env, "existing")
}

func (m *Exec) start(env []string, state string) error {
	return syscall.Exec(m.EtcdBinary,
		[]string{"--initial-cluster-state=" + state},
		env,
	)
}

func (m *Exec) Stop() error {
	return nil
}

func (m *Exec) Wait() error {
	return nil
}
