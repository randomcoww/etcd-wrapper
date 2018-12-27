package wrapper

import (
	"time"

	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/sirupsen/logrus"
)

type PodConfig struct {
	config *config.Config
	stop   chan struct{}
}

func newPodConfig(c *config.Config) *PodConfig {
	return &PodConfig{
		config: c,
		stop:   make(chan struct{}, 1),
	}
}

func (p *PodConfig) runCreateNew() {
	fetchSuccess := false

	for {
		select {
		case <-time.After(p.config.PodUpdateInterval):
			// This is retried every time if it failed
			if !fetchSuccess {
				if err := fetchBackup(p.config); err == nil {
					fetchSuccess = true
				}
			}

			if fetchSuccess {
				// Start existing with snapshot restore
				writePodSpec(p.config, "existing", true)
			} else {
				// Start new with no data
				writePodSpec(p.config, "new", false)
			}
		case <-p.stop:
			return
		}
	}
}

func (p *PodConfig) runCreateExisting() {
	for {
		select {
		case <-time.After(p.config.PodUpdateInterval):
			// Start existing with no data
			writePodSpec(p.config, "existing", false)

		case <-p.stop:
			return
		}
	}
}

func (p *PodConfig) stopRun() {
	select {
	case p.stop <- struct{}{}:
	default:
	}
}

func fetchBackup(c *config.Config) error {
	err := backup.FetchBackup(c.S3BackupPath, c.BackupFile)
	switch err {
	case nil:
		logrus.Infof("Fetch snapshot success")
		return nil
	default:
		logrus.Errorf("Fetch snapshot failed: %v", err)
		return err
	}
}

func writePodSpec(c *config.Config, state string, restore bool) {
	c.UpdateInstance()
	config.WritePodSpec(config.NewEtcdPod(c, state, restore), c.PodSpecFile)
	logrus.Errorf("Write pod spec: (state: %v, restore: %v)", state, restore)
}
