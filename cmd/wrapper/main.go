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
		err = v.UpdateFromList()
		if err != nil {

		}

		err = v.UpdateFromStatus()
		if err != nil {

		}

		


		// local member is running?
		if v.MemberSelf.Healthy {
			if v.HasSplitBrain() {
				// restart as existing
			}
			// do nothing
		} else {


			// check if rest of cluster is healthy
			// start as existing with no restore
		}
	}
}