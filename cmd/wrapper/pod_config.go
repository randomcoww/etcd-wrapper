package wrapper

import (
	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/sirupsen/logrus"
	"os"
)

type PodConfig struct {
	config *config.Config
}

func newPodConfig(c *config.Config) *PodConfig {
	return &PodConfig{
		config: c,
	}
}

// Pull backup if possible, and check if snapshot file was generated
func (p *PodConfig) checkBackup() (bool, error) {
	logrus.Infof("[podconfig] Fetching snapshot")
	if err := backup.FetchBackup(p.config.S3BackupPath, p.config.BackupFile); err != nil {
		logrus.Warningf("[podconfig] Fetch snapshot failed: %v", err)
		return false, err
	}
	logrus.Infof("[podconfig] Fetched snapshot")

	if f, err := os.Stat(p.config.BackupFile); err != nil {
		if os.IsNotExist(err) {
			logrus.Warningf("[podconfig] Snapshot file does not exist")
			return false, nil
		}
		if f.Size() == 0 {
			logrus.Warningf("[podconfig] Snapshot file is empty")
			return false, nil
		}
		logrus.Errorf("[podconfig] Failed to stat snapshot file: %v", err)
		return false, err
	}
	return true, nil
}

func (p *PodConfig) createForNewCluster() {
	logrus.Infof("[podconfig] Create manifest for new cluster")
	if ok, _ := p.checkBackup(); ok {
		writePodSpec(p.config, "existing", true)
	} else {
		writePodSpec(p.config, "new", false)
	}
}

func (p *PodConfig) createForExistingCluster() {
	logrus.Infof("[podconfig] Create manifest for existing cluster")
	writePodSpec(p.config, "existing", false)
}

func writePodSpec(c *config.Config, state string, restore bool) {
	if err := config.WritePodSpec(config.NewEtcdPod(c, state, restore), c.PodSpecFile); err != nil {
		logrus.Errorf("[podconfig] Failed to write pod spec: (state: %v, restore: %v): %v", state, restore, err)
		return
	}
	logrus.Infof("[podconfig] Wrote pod spec: (state: %v, restore: %v)", state, restore)
}
