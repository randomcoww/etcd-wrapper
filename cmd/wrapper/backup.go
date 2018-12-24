package wrapper

import (
	"crypto/tls"
	"time"

	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/cluster"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
)

func runBackupHandler(c *cluster.Cluster, clientURLs []string, tlsConfig *tls.Config) {
	logrus.Infof("Start backup handler")
	for {
		select {
		case <-time.After(c.BackupInterval):
			status, err := etcdutilextra.Status(clientURLs, tlsConfig)
			if err != nil {
				logrus.Errorf("Failed to get status: %v", err)
				continue
			}

			// Run backup if this node is the leader
			if c.ID == status.Leader {
				logrus.Infof("Start backup process")
				err := backup.SendBackup(c.S3BackupPath, tlsConfig, clientURLs)

				if err != nil {
					logrus.Errorf("Backup failed: %v", err)
				} else {
					logrus.Infof("Finished backup")
				}
				logrus.Infof("Backup handler sleep for %v", c.BackupInterval)
			}
		}
	}
}
