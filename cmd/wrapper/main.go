package wrapper

import (
	"os"
	"time"

	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
)

func Main() {
	c, err := config.NewConfig()
	if err != nil {
		logrus.Errorf("Parse config failed: %v", err)
		os.Exit(1)
	}

	backup := newBackup(c)
	podConfig := newPodConfig(c)
	memberStatus := newMemberStatus(c)

	go backup.runPeriodic()

	for {
		podConfig.runCreateNew()
	checkCluster:
		for {
			select {
			case <-time.After(c.HealthCheckInterval):
				memberList, err := etcdutil.ListMembers(c.ClientURLs, c.TLSConfig)
				if err != nil {
					logrus.Errorf("Cluster healthcheck failed: %v", err)

				} else {
					podConfig.stopRun()
					memberStatus.mergeMemberList(memberList)
					break checkCluster
				}
			}
		}

		podConfig.runCreateExisting()
	checkLocal:
		for {
			select {
			case <-time.After(c.HealthCheckInterval):
				err := etcdutilextra.HealthCheck(c.ClientURLs, c.TLSConfig)
				if err != nil {
					logrus.Errorf("Cluster healthcheck failed: %v", err)
					memberStatus.removeLocalMember()
					memberStatus.addLocalMember()

				} else {
					podConfig.stopRun()
					break checkLocal
				}
			}
		}

	}
}
