package wrapper

import (
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/pkg/transport"
	"os"
	"time"
)

func main() {
	status, err := newStatus()
	if err != nil {
		panic(err)
	}

	for {
		select {
		case d := <-podRestartWait:
			time.Sleep(d)

		case time.Tick():
			err = v.SyncStatus()
			if err != nil {
				panic(err)
			}
			if v.ClusterID == nil {
				// no cluster ID found
				// run restore
				v.StartEtcdPod(true)
				podRestartWait <- 2*time.Minute
				break
			}

			if !v.Healthy {
				v.StartEtcdPod(true)
				podRestartWait <- 2*time.Minute
				break
			}

			if v.MemberSelf.ClusterID != v.ClusterID {
				if v.MemberSelf.MemberIDFromCluster != v.MemberSelf.MemberID {
					// do add remove
					err := v.ReplaceMember()
					if err != nil {
						v.StartEtcdPod(true)
						podRestartWait <- 2*time.Minute
						break
					}
					v.StartEtcdPod(false)
					podRestartWait <- 2*time.Minute
					break
				}

				// this should never happen
				v.StartEtcdPod(true)
				podRestartWait <- 2*time.Minute
				break
			}

			// handle backup
		}
	}
}