// Test EtcdExec with fork process that can be stopped

package etcd

import (
	"context"
	"os"
	"os/exec"
)

type Fork struct {
	Cmd        *exec.Cmd
	Ctx        context.Context
	EtcdBinary string
}

func (m *Fork) StartNew(env []string) error {
	m.Cmd = exec.CommandContext(m.Ctx, m.EtcdBinary)
	m.Cmd.Args = []string{
		m.EtcdBinary,
		"--initial-cluster-state",
		"new",
	}
	m.Cmd.Env = env
	m.Cmd.Stdout = os.Stdout
	m.Cmd.Stderr = os.Stderr
	return m.Cmd.Start()
}

func (m *Fork) StartExisting(env []string) error {
	m.Cmd = exec.CommandContext(m.Ctx, m.EtcdBinary)
	m.Cmd.Args = []string{
		m.EtcdBinary,
		"--initial-cluster-state",
		"existing",
	}
	m.Cmd.Env = env
	m.Cmd.Stdout = os.Stdout
	m.Cmd.Stderr = os.Stderr
	return m.Cmd.Start()
}

func (m *Fork) Stop() error {
	if m.Cmd.Process != nil {
		return m.Cmd.Process.Kill()
	}
	return nil
}

func (m *Fork) Wait() error {
	if m.Cmd.Process != nil {
		_, err := m.Cmd.Process.Wait()
		return err
	}
	return nil
}
