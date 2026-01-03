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

func RestoreV3Snapshot(ctx context.Context, config *c.Config, snapshotFile string, versionBump uint64) error {
	c := exec.CommandContext(ctx, config.EtcdutlBinaryFile)
	c.Args = []string{
		config.EtcdutlBinaryFile,
		"snapshot", "restore", snapshotFile,
		"--name", config.Env["ETCD_NAME"],
		"--initial-cluster", config.Env["ETCD_INITIAL_CLUSTER"],
		"--initial-cluster-token", config.Env["ETCD_INITIAL_CLUSTER_TOKEN"],
		"--initial-advertise-peer-urls", config.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"],
		"--data-dir", config.Env["ETCD_DATA_DIR"],
		"--bump-revision", fmt.Sprintf("%d", versionBump),
	}
	if d, ok := config.Env["ETCD_WAL_DIR"]; ok && d != "" {
		c.Args = append(c.Args, "--wal-dir", d)
	}
	if versionBump > 0 {
		c.Args = append(c.Args, "--mark-compacted")
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
