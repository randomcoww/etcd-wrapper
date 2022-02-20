package wrapper

import (
	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
	"time"
)

type Backup struct {
	config *config.Config
	stop   chan struct{}
}

func newBackup(c *config.Config) *Backup {
	return &Backup{
		config: c,
		stop:   make(chan struct{}, 1),
	}
}

func (b *Backup) runPeriodic() {
	logrus.Infof("[backup] Start periodic")

	for {
		select {
		case <-time.After(b.config.BackupInterval):
			logrus.Infof("[backup] Start run")

			status, err := etcdutil.Status(b.config.LocalClientURLs, b.config.TLSConfig)
			if err != nil {
				logrus.Errorf("[backup] Etcd status lookup failed: %v", err)
				continue
			}

			// Only local client URL is hit
			// Responding member should always be my node
			if status.Header.MemberId == status.Leader {
				err := backup.SendBackup(b.config.S3BackupPath, b.config.TLSConfig, b.config.ClientURLs)
				if err != nil {
					logrus.Errorf("[backup] Backup failed: %v", err)
				} else {
					logrus.Infof("[backup] Backup succeeded")
				}
			} else {
				logrus.Infof("[backup] Skipping backup from non-leader")
			}
		case <-b.stop:
			logrus.Infof("[backup] Stop periodic")
			return
		}
	}
}

func (b *Backup) stopRun() {
	select {
	case b.stop <- struct{}{}:
	default:
	}
}
