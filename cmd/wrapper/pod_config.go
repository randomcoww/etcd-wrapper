package wrapper

import (
	"os"
	"time"

	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/sirupsen/logrus"
)

type PodConfig struct {
	config     *config.Config
	allowFetch chan struct{}
}

func newPodConfig(c *config.Config) *PodConfig {
	p := &PodConfig{
		config:     c,
		allowFetch: make(chan struct{}, 1),
	}

	// Allow fetching backup initially and after every backup interval
	p.allowFetch <- struct{}{}
	go func() {
		for {
			select {
			case <-time.After(p.config.BackupInterval):
				select {
				case p.allowFetch <- struct{}{}:
				default:
				}
			}
		}
	}()
	return p
}

// Pull backup if possible, and check if snapshot file was generated
func (p *PodConfig) checkBackup() (bool, error) {
	select {
	case <-p.allowFetch:
		logrus.Infof("[podconfig] Fetching snapshot")
		if err := backup.FetchBackup(p.config.S3BackupPath, p.config.BackupFile); err != nil {
			logrus.Warningf("[podconfig] Fetch snapshot failed: %v", err)
		} else {
			logrus.Infof("[podconfig] Fetch snapshot succeeded")
		}
	default:
		logrus.Warningf("[podconfig] Throttling fetching snapshot")
	}

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
	c.UpdateInstance()
	if err := config.WritePodSpec(config.NewEtcdPod(c, state, restore), c.PodSpecFile); err != nil {
		logrus.Errorf("[podconfig] Failed to write pod spec: (state: %v, restore: %v): %v", state, restore, err)
		return
	}
	logrus.Infof("[podconfig] Wrote pod spec: (state: %v, restore: %v)", state, restore)
}
