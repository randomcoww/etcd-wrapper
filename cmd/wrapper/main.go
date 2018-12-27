package wrapper

import (
	"os"
	"time"

	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
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
	go backup.runPeriodic()

	podConfig := newPodConfig(c)
	go podConfig.periodicTriggerFetch()

	memberStatus := newMemberStatus(c)
	var updateState int

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

				err = etcdutilextra.HealthCheck(c.LocalClientURLs, c.TLSConfig)
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
