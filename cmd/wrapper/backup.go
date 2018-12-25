package wrapper

import (
	"time"

	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
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
	logrus.Infof("Start backup handler")

	for {
		select {
		case <-time.After(b.config.BackupInterval):
			status, err := etcdutilextra.Status(b.config.LocalClientURLs, b.config.TLSConfig)
			if err != nil {
				logrus.Errorf("Failed to get status: %v", err)
				continue
			}

			// Only local client URL is hit
			// Responding member should always be my node
			if status.Header.MemberId == status.Leader {
				logrus.Infof("Start backup")

				err := backup.SendBackup(b.config.S3BackupPath, b.config.TLSConfig, b.config.LocalClientURLs)
				if err != nil {
					logrus.Errorf("Backup failed: %v", err)
				} else {
					logrus.Infof("Finished backup")
				}
			}
		case <-b.stop:
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
