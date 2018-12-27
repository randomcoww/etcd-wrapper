package wrapper

import (
	"os"
	"time"

	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/sirupsen/logrus"
)

type PodConfig struct {
	config       *config.Config
	fetchTrigger chan struct{}
	fetchExists  bool
}

func newPodConfig(c *config.Config) *PodConfig {
	p := &PodConfig{
		config:       c,
		fetchTrigger: make(chan struct{}, 1),
	}
	p.triggerFetch()
	return p
}

func (p *PodConfig) triggerFetch() {
	select {
	case p.fetchTrigger <- struct{}{}:
	default:
	}
}

func (p *PodConfig) periodicTriggerFetch() {
	for {
		select {
		case <-time.After(p.config.BackupInterval):
			p.triggerFetch()
		}
	}
}

func (p *PodConfig) fetchBackup() {
	if err := backup.FetchBackup(p.config.S3BackupPath, p.config.BackupFile); err != nil {
		logrus.Errorf("Fetch snapshot failed: %v", err)
		p.fetchExists = false
	} else {
		logrus.Infof("Fetch snapshot success")
		p.fetchExists = true
	}
}

func (p *PodConfig) checkFetchFileExists() {
	if _, err := os.Stat(p.config.BackupFile); os.IsNotExist(err) {
		p.fetchExists = false
	}
}

func (p *PodConfig) createForNewCluster() {
	// Sets minimum interval for refetching backup from remote
	select {
	case <-p.fetchTrigger:
		p.fetchBackup()
	default:
	}
	p.checkFetchFileExists()

	if p.fetchExists {
		// Start existing with snapshot restore
		writePodSpec(p.config, "existing", true)
	} else {
		// Start new with no data
		writePodSpec(p.config, "new", false)
	}
}

func (p *PodConfig) createForExistingCluster() {
	writePodSpec(p.config, "existing", false)
}

func writePodSpec(c *config.Config, state string, restore bool) {
	c.UpdateInstance()
	config.WritePodSpec(config.NewEtcdPod(c, state, restore), c.PodSpecFile)
	logrus.Errorf("Write pod spec: (state: %v, restore: %v)", state, restore)
}
