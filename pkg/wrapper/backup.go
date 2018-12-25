package wrapper

import (
	"crypto/tls"
	"time"

	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
)

type Backup struct {
	interval   time.Duration
	clientURLs []string
	tc         *tls.Config
	s3Path     string
	stop       chan struct{}
}

func NewBackup(clientURLs []string, tc *tls.Config, interval time.Duration, s3Path string) {
	return &Backup{
		interval:   interval,
		clientURLs: clientURLs,
		tc:         tc,
		s3Path:     s3Path,
		stop:       make(chan struct{}, 1),
	}
}

func (c *Backup) RunPeriodic() {
	logrus.Infof("Start backup handler")

	for {
		select {
		case <-time.After(c.interval):
			status, err := etcdutilextra.Status(c.clientURLs, c.tc)
			if err != nil {
				logrus.Errorf("Failed to get status: %v", err)
				continue
			}

			// Only local client URL is hit
			// Responding member should always be my node
			if status.Header.MemberId == status.Leader {
				logrus.Infof("Start backup")

				err := backup.SendBackup(c.s3Path, c.tc, c.clientURLs)
				if err != nil {
					logrus.Errorf("Backup failed: %v", err)
				} else {
					logrus.Infof("Finished backup")
				}
			}
		case <-c.stop:
			return
		}
	}
}

func (c *Backup) Stop() {
	select {
	case c.stop <- struct{}{}:
	default:
	}
}
