// Test EtcdExec with fork process that can be stopped

package etcd

import (
	"context"
	"os"
	"os/exec"

	c "github.com/randomcoww/etcd-wrapper/internal/config"
)

type Fork struct {
	Cmd *exec.Cmd
	Ctx context.Context
}

func (m *Fork) StartNew(config *c.Config) error {
	m.Cmd = exec.CommandContext(m.Ctx, config.EtcdBinaryFile)
	m.Cmd.Args = []string{
		config.EtcdBinaryFile,
		"--initial-cluster-state",
		"new",
	}
	m.Cmd.Env = config.WriteEnv()
	m.Cmd.Stdout = os.Stdout
	m.Cmd.Stderr = os.Stderr
	return m.Cmd.Start()
}

func (m *Fork) StartExisting(config *c.Config) error {
	m.Cmd = exec.CommandContext(m.Ctx, config.EtcdBinaryFile)
	m.Cmd.Args = []string{
		config.EtcdBinaryFile,
		"--initial-cluster-state",
		"existing",
	}
	m.Cmd.Env = config.WriteEnv()
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
