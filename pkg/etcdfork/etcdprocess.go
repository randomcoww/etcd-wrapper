// Test EtcdExec with fork process that can be stopped

package etcdfork

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"os"
	"os/exec"
)

type EtcdFork struct {
	Cmd *exec.Cmd
	Ctx context.Context
}

func (m *EtcdFork) StartNew(config *c.Config) error {
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

func (m *EtcdFork) StartExisting(config *c.Config) error {
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

func (m *EtcdFork) Stop() error {
	if m.Cmd.Process != nil {
		return m.Cmd.Process.Kill()
	}
	return nil
}

func (m *EtcdFork) Wait() error {
	if m.Cmd.Process != nil {
		_, err := m.Cmd.Process.Wait()
		return err
	}
	return nil
}
