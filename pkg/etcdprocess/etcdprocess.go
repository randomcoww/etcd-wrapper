package etcdprocess

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"os"
	"os/exec"
	"syscall"
)

type EtcdProcess interface {
	StartEtcdNew(*c.Config) error
	StartEtcdExisting(*c.Config) error
	Stop() error
	Wait() error
}

type etcdProcess struct {
}

func NewEtcdProcess() *etcdProcess {
	return &etcdProcess{}
}

func (*etcdProcess) StartEtcdNew(config *c.Config) error {
	return syscall.Exec(config.EtcdBinaryFile, []string{
		config.EtcdBinaryFile,
		"--initial-cluster-state",
		"new",
	}, config.WriteEnv())
}

func (*etcdProcess) StartEtcdExisting(config *c.Config) error {
	return syscall.Exec(config.EtcdBinaryFile, []string{
		config.EtcdBinaryFile,
		"--initial-cluster-state",
		"existing",
	}, config.WriteEnv())
}

func (*etcdProcess) Stop() error {
	return nil
}

func (*etcdProcess) Wait() error {
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
