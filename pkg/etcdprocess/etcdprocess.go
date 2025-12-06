package etcdprocess

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
)

type EtcdProcess interface {
	Start() error
	Stop() error
	Wait() error
	Reconfigure(*c.Config) error
}

type etcdProcess struct {
	*exec.Cmd
}

func NewProcess(ctx context.Context, config *c.Config) EtcdProcess {
	cmd := exec.CommandContext(ctx, config.EtcdBinaryFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = config.WriteEnv()

	return &etcdProcess{
		Cmd: cmd,
	}
}

func (p *etcdProcess) Reconfigure(config *c.Config) error {
	env := config.WriteEnv()
	if !slices.Equal(p.Cmd.Env, env) {
		if err := p.Stop(); err != nil {
			return err
		}
	}
	copy(p.Cmd.Env, env)
	return p.Start()
}

func (p *etcdProcess) Start() error {
	if p.Cmd.Process == nil {
		return p.Cmd.Start()
	}
	return nil
}

func (p *etcdProcess) Stop() error {
	if p.Cmd.Process != nil {
		if err := p.Cmd.Process.Kill(); err != nil {
			return err
		}
		return p.Wait()
	}
	return nil
}

func (p *etcdProcess) Wait() error {
	if p.Cmd.Process != nil {
		_, err := p.Cmd.Process.Wait()
		return err
	}
	return nil
}

func RestoreV3Snapshot(ctx context.Context, config *c.Config, snapshotFile string) error {
	if err := RemoveDataDir(config); err != nil {
		return err
	}
	c := exec.CommandContext(ctx, config.EtcdutlBinaryFile)
	c.Args = append(c.Args, "snapshot", "restore", snapshotFile)
	c.Args = append(c.Args, "--name", config.Env["ETCD_NAME"])
	c.Args = append(c.Args, "--initial-cluster", config.Env["ETCD_INITIAL_CLUSTER"])
	c.Args = append(c.Args, "--initial-cluster-token", config.Env["ETCD_INITIAL_CLUSTER_TOKEN"])
	c.Args = append(c.Args, "--initial-advertise-peer-urls", config.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"])
	c.Args = append(c.Args, "--data-dir", config.Env["ETCD_DATA_DIR"])
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

func DataExists(config *c.Config) (bool, error) {
	info, err := os.Stat(config.Env["ETCD_DATA_DIR"])
	if err != nil {
		return false, err
	}
	switch {
	case info.IsDir():
		paths, err := os.ReadDir(config.Env["ETCD_DATA_DIR"])
		if err != nil {
			return false, err
		}
		return len(paths) > 0, nil
	default:
		return false, fmt.Errorf("ETCD_DATA_DIR is not a directory")
	}
}

func RemoveDataDir(config *c.Config) error {
	info, err := os.Stat(config.Env["ETCD_DATA_DIR"])
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	}
	switch {
	case info.IsDir():
		paths, err := os.ReadDir(config.Env["ETCD_DATA_DIR"])
		if err != nil {
			return err
		}
		for _, path := range paths {
			if err := os.RemoveAll(filepath.Join(config.Env["ETCD_DATA_DIR"], path.Name())); err != nil {
				return err
			}
		}
	default:
		if err := os.RemoveAll(config.Env["ETCD_DATA_DIR"]); err != nil {
			return err
		}
	}
	return nil
}
