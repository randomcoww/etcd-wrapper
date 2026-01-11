package s3backup

import (
	"context"
	"fmt"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"go.uber.org/zap"
	"io"
	"os"
	"os/exec"
	"time"
)

func (c *client) RestoreSnapshot(ctx context.Context, config *c.Config, versionBump uint64) (bool, error) {
	objects, err := c.list(ctx, config)
	if err != nil {
		return false, err
	}
	if len(objects) == 0 {
		return false, nil
	}
	var ok bool
	for i := len(objects) - 1; i >= 0; i-- {
		ok, err = c.restoreSnapshotKey(ctx, config, objects[i].Key, versionBump)
		if err == nil && ok {
			config.Logger.Info("restored snapshot success")
			break
		}
		config.Logger.Error("restore failed", zap.Error(err))
	}
	if err != nil {
		return false, fmt.Errorf("all restore failed %w", err)
	}
	return true, nil
}

func (c *client) restoreSnapshotKey(ctx context.Context, config *c.Config, key string, versionBump uint64) (bool, error) {
	config.Logger.Info("attempting snapshot restore")
	ctx, _ = context.WithTimeout(ctx, time.Duration(config.RestoreTimeout))

	dir, err := os.MkdirTemp("", "etcd-wrapper-*")
	if err != nil {
		config.Logger.Error("create path for snapshot failed", zap.Error(err))
		return false, err
	}
	defer os.RemoveAll(dir)

	snapshotFile, err := os.CreateTemp(dir, "snapshot-restore-*.db")
	if err != nil {
		config.Logger.Error("open file for snapshot failed", zap.Error(err))
		return false, err
	}
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()
	config.Logger.Info("opened file for snapshot")

	ok, err := c.download(ctx, config, key, func(ctx context.Context, reader io.Reader) error {
		b, err := io.Copy(snapshotFile, reader)
		if err != nil {
			return err
		}
		if b == 0 {
			return fmt.Errorf("snapshot file download size was 0")
		}
		return nil
	})
	if err != nil {
		config.Logger.Error("download snapshot failed", zap.Error(err))
		return false, err
	}
	if !ok {
		config.Logger.Info("no snapshots found")
		return false, nil
	}
	if err := restoreV3Snapshot(ctx, config, snapshotFile.Name(), versionBump); err != nil {
		config.Logger.Error("restore snapshot failed", zap.Error(err))
		return false, err
	}
	config.Logger.Info("finished restoring snapshot")
	return true, nil
}

func restoreV3Snapshot(ctx context.Context, config *c.Config, snapshotFile string, versionBump uint64) error {
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
