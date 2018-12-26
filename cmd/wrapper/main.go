package wrapper

import (
	"os"

	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
)

func Main() {
	c, err := config.NewConfig()
	if err != nil {
		logrus.Errorf("Parse config failed: %v", err)
		os.Exit(1)
	}

	healthcheck := newHealthCheck(c)
	backup := newBackup(c)

	// go healthcheck.runLocalCheck()
	go healthcheck.runClusterCheck()
	go backup.runPeriodic()

	run(c)
}

func run(c *config.Config) {
	for {
		select {
			// cluster err
		case <-c.NotifyMissingNew:
			err := fetchBackup(c)
			if err != nil {
				// Start new with no data
				writePodSpec(c, "new", false)
			} else {
				// Start existing with snapshot restore
				writePodSpec(c, "existing", true)
			}

			// local err
		case <-c.NotifyMissingExisting:
			// Create pod spec with existing
			writePodSpec(c, "existing", false)

		case memberID := <-c.NotifyLocalRemove:
			removeMember(c, memberID)
			
		case memberID := <-c.NotifyRemoteRemove:
			removeMember(c, memberID)

		case <-c.NotifyLocalAdd:
			addMember(c)
		}
	}
}

func fetchBackup(c *config.Config) error {
	err := backup.FetchBackup(c.S3BackupPath, c.BackupFile)
	switch err {
	case nil:
		logrus.Infof("Fetch snapshot success")
		return nil
	default:
		logrus.Errorf("Fetch snapshot failed: %v", err)
		return err
	}
}

func writePodSpec(c *config.Config, state string, restore bool) {
	config.WritePodSpec(config.NewEtcdPod(c, state, restore), c.PodSpecFile)
	logrus.Errorf("Write pod spec: (state: %v, restore: %v)", state, restore)
}

func addMember(c *config.Config) error {
	resp, err := etcdutilextra.AddMember(c.ClientURLs, c.LocalPeerURLs, c.TLSConfig)
	switch err {
	case nil:
		logrus.Infof("Add member success: %v", resp.Member.ID)
		return nil
	case rpctypes.ErrMemberExist:
		logrus.Infof("Add member already exists")
		return nil
	default:
		logrus.Errorf("Add member failed: %v", err)
		return err
	}
}

func removeMember(c *config.Config, memberID uint64) error {
	err := etcdutil.RemoveMember(c.ClientURLs, c.TLSConfig, memberID)
	switch err {
	case nil:
		logrus.Infof("Remove member success: %v", memberID)
		return nil
	case rpctypes.ErrMemberNotFound:
		logrus.Infof("Remove member not found: %v", memberID)
		return nil
	default:
		logrus.Errorf("Remove member failed (%v): %v", memberID, err)
		return err
	}
}
