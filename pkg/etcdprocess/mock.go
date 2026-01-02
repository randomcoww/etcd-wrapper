package etcdprocess

import (
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"os/exec"
)

type mockEtcdProcess struct {
	*exec.Cmd
}

func NewMockEtcdProcess() *mockEtcdProcess {
	return &mockEtcdProcess{}
}

func (m *mockEtcdProcess) StartEtcdNew(config *c.Config) error {
	m.Cmd = exec.Command(config.EtcdBinaryFile)
	m.Cmd.Env = config.WriteEnv()
	m.Cmd.Args = append(m.Cmd.Args, "--initial-cluster-state", "new")
	return m.Cmd.Start()
}

func (m *mockEtcdProcess) StartEtcdExisting(config *c.Config) error {
	m.Cmd = exec.Command(config.EtcdBinaryFile)
	m.Cmd.Env = config.WriteEnv()
	m.Cmd.Args = append(m.Cmd.Args, "--initial-cluster-state", "existing")
	return m.Cmd.Start()
}

func (m *mockEtcdProcess) Stop() error {
	return m.Cmd.Process.Kill()
}

func (m *mockEtcdProcess) Wait() error {
	_, err := m.Cmd.Process.Wait()
	return err
}
