// Test EtcdExec with fork process that can be stopped

package etcd

import (
	"context"
	"os"
	"os/exec"
)

type Fork struct {
	Cmd        *exec.Cmd
	EtcdBinary string
	cancel     func()
}

func (m *Fork) StartNew(ctx context.Context, env []string) error {
	return m.start(ctx, env, "new")
}

func (m *Fork) StartExisting(ctx context.Context, env []string) error {
	return m.start(ctx, env, "existing")
}

// Create a new instance each time to allow restart test
func (m *Fork) start(ctx context.Context, env []string, state string) error {
	ctx, cancel := context.WithCancel(ctx)
	m.Cmd = exec.CommandContext(ctx, m.EtcdBinary, "--initial-cluster-state", state)
	m.Cmd.Env = env
	m.Cmd.Stdout = os.Stdout
	m.Cmd.Stderr = os.Stderr
	m.cancel = cancel
	return m.Cmd.Start()
}

func (m *Fork) Stop() error {
	m.cancel()
	if m.Cmd.Process != nil {
		_, err := m.Cmd.Process.Wait()
		return err
	}
	return nil
}
