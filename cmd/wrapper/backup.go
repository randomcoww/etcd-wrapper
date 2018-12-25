package wrapper

import (
	"crypto/tls"
	"time"

	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/cluster"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
)

func runBackupHandler(c *cluster.Cluster, tlsConfig *tls.Config) {
	localClientULRs := cluster.LocalClientURLsFromConfig(c)

	logrus.Infof("Start backup handler")
	for {
		select {
		case <-time.After(c.BackupInterval):
			status, err := etcdutilextra.Status(localClientULRs, tlsConfig)
			if err != nil {
				logrus.Errorf("Failed to get status: %v", err)
				continue
			}

			// Only local instance is hit - responding member should always be my node
			// Check if leader
			if status.Header.MemberId == status.Leader {
				logrus.Infof("Start backup process")

				err := backup.SendBackup(c.S3BackupPath, tlsConfig, localClientULRs)
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
