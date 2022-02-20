package wrapper

import (
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
	"os"
	"time"
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

	clusterUpdateState := newClusterUpdateState(c.HealthCheckFailuresAllow)
	stop := make(chan bool)

	go backup.runPeriodic()

	go func() {
		for {
			select {
			case <-time.After(c.HealthCheckInterval):
				// Check if other cluster members are up
				memberList, err := etcdutil.ListMembers(c.ClientURLs, c.TLSConfig)
				if err != nil {
					clusterUpdateState.setState(updateStateNewCluster)
					logrus.Warningf("[memberstatus] Cluster healthcheck failed (%d): %v", clusterUpdateState.counter, err)
					continue
				}
				// Create updated memberlist
				memberStatus.mergeMemberList(memberList)

				// Cluster is up - now check if this local member is up
				err = etcdutil.HealthCheck(c.LocalClientURLs, c.TLSConfig)
				if err != nil {
					clusterUpdateState.setState(updateStateExistingCluster)
					logrus.Warningf("[memberstatus] Local member healthcheck failed (%d): %v", clusterUpdateState.counter, err)
					continue
				}

				clusterUpdateState.clearState()
			}
		}
	}()

	go func() {
		for {
			select {
			case <-time.After(c.PodUpdateWait):
				// Block on next updateState
				switch <-clusterUpdateState.ch {
				case updateStateNewCluster:
					podConfig.createForNewCluster()

				case updateStateExistingCluster:
					podConfig.createForExistingCluster()
					memberStatus.removeLocalMember()
					memberStatus.addLocalMember()
				}
			}
		}
	}()

	<-stop
}
