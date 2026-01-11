package etcdprocess

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"os"
	"os/exec"
)

type EtcdProcess interface {
	StartEtcdNew(context.Context, *c.Config) error
	StartEtcdExisting(context.Context, *c.Config) error
	Stop() error
	Wait() error
}

type etcdProcess struct {
	*exec.Cmd
}

func NewEtcdProcess() *etcdProcess {
	return &etcdProcess{}
}

func (m *etcdProcess) StartEtcdNew(ctx context.Context, config *c.Config) error {
	m.Cmd = exec.CommandContext(ctx, config.EtcdBinaryFile)
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

func (m *etcdProcess) StartEtcdExisting(ctx context.Context, config *c.Config) error {
	m.Cmd = exec.CommandContext(ctx, config.EtcdBinaryFile)
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

func (m *etcdProcess) Stop() error {
	if m.Cmd.Process != nil {
		return m.Cmd.Process.Kill()
	}
	return nil
}

func (m *etcdProcess) Wait() error {
	if m.Cmd.Process != nil {
		_, err := m.Cmd.Process.Wait()
		return err
	}
	return nil
}
