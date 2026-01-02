package etcdprocess

import (
	"context"
	"fmt"
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

func RestoreV3Snapshot(ctx context.Context, config *c.Config, snapshotFile string) error {
	c := exec.CommandContext(ctx, config.EtcdutlBinaryFile)
	c.Args = append(c.Args, "snapshot", "restore", snapshotFile)
	c.Args = append(c.Args, "--name", config.Env["ETCD_NAME"])
	c.Args = append(c.Args, "--initial-cluster", config.Env["ETCD_INITIAL_CLUSTER"])
	c.Args = append(c.Args, "--initial-cluster-token", config.Env["ETCD_INITIAL_CLUSTER_TOKEN"])
	c.Args = append(c.Args, "--initial-advertise-peer-urls", config.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"])
	c.Args = append(c.Args, "--data-dir", config.Env["ETCD_DATA_DIR"])
	if d, ok := config.Env["ETCD_WAL_DIR"]; ok && d != "" {
		c.Args = append(c.Args, "--wal-dir", d)
	}
	c.Env = config.WriteEnv()
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Start(); err != nil {
		return err
	}
	processState, err := c.Process.Wait()
	if err != nil {
		return err
	}
	if !processState.Success() {
		return fmt.Errorf("etcdutl snapshot restore returned a non-zero exit code")
	}
	return nil
}
