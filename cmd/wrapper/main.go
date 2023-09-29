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

	var waitCount int

	for {
		select {

		case time.Tick(6 * time.Second):
			if waitCount > 0 {
				waitCount--
				continue
			}

			err = v.SyncStatus()
			if err != nil {
				panic(err)
			}
			if v.ClusterID == nil {
				// no cluster ID found
				// run restore
				v.StartEtcdPod(true)
				// podRestartWait <- 2 * time.Minute
				waitCount = 10
				break
			}

			if !v.Healthy {
				v.StartEtcdPod(true)
				waitCount = 10
				break
			}

			if v.MemberSelf.ClusterID != v.ClusterID {
				if v.MemberSelf.MemberIDFromCluster != v.MemberSelf.MemberID {
					// do add remove
					err := v.ReplaceMember(v.MemberSelf)
					if err != nil {
						v.StartEtcdPod(true)
						waitCount = 10
						break
					}
					v.StartEtcdPod(false)
					waitCount = 10
					break
				}

				// this should never happen
				v.StartEtcdPod(true)
				waitCount = 10
				break
			}
			// handle backup
		}
	}
}
