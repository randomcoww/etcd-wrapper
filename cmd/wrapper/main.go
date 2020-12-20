package wrapper

import (
	"os"
	"time"

	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
)

const (
	updateStateNewCluster      = 1
	updateStateExistingCluster = 2
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
	var updateState int

	go backup.runPeriodic()

	for {
		updateTimer := time.After(c.PodUpdateInterval)
	healthCheck:
		for {
			select {
			case <-time.After(c.HealthCheckInterval):
				memberList, err := etcdutil.ListMembers(c.ClientURLs, c.TLSConfig)
				if err != nil {
					logrus.Errorf("Cluster healthcheck failed: %v", err)
					updateState = updateStateNewCluster
					continue
				}
				memberStatus.mergeMemberList(memberList)

				err = etcdutil.HealthCheck(c.LocalClientURLs, c.TLSConfig)
				if err != nil {
					logrus.Errorf("Local healthcheck failed: %v", err)
					updateState = updateStateExistingCluster
					continue
				}
				// Success
				break healthCheck

			case <-updateTimer:
				switch updateState {
				case updateStateNewCluster:
					podConfig.createForNewCluster()

				case updateStateExistingCluster:
					podConfig.createForExistingCluster()
					memberStatus.removeLocalMember()
					memberStatus.addLocalMember()
				default:
				}
				// Reset timers after resource update
				break healthCheck
			}
		}
	}
}
