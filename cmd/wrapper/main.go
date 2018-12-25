package wrapper

import (
	// "crypto/tls"
	// "io/ioutil"
	// "time"

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
		logrus.Errorf("Failed to parse config")
		return
	}

	healthcheck := newHealthCheck(c)
	backup := newBackup(c)

	go healthcheck.runLocalCheck()
	go healthcheck.runClusterCheck()
	go backup.runPeriodic()

	run(c)
}

func run(c *config.Config) {
	for {
		select {
		case <-c.NotifyMissingNew:
			// Recover member from backup or create new
			err := fetchBackup(c)
			if err != nil {
				// Start new with no data
				writePodSpec(c, "new", false)
			} else {
				// Start existing with snapshot restore
				writePodSpec(c, "existing", true)
			}

		case memberID := <-c.NotifyRemoteRemove:
			// Remove this member
			removeMember(c, memberID)

		case memberID := <-c.NotifyLocalRemove:
			// Remove local member
			err := removeMember(c, memberID)
			if err != nil {
				// Remove local member failed - cluster issue?
			} else {
				//
			}

		case <-c.NotifyMissingExisting:
			// Add local member as existing with blank data
			err := addMember(c)
			if err != nil {
				// Add member failed - cluster issue?
			} else {
				// Start existing with no data
				writePodSpec(c, "existing", false)
			}
		}
	}
}

func fetchBackup(c *config.Config) error {
	err := backup.FetchBackup(c.S3BackupPath, c.BackupFile)
	switch err {
	case nil:
		logrus.Infof("Fetched snapshot backup")
		return nil
	default:
		logrus.Errorf("Failed to fetch backup: %v", err)
		return err
	}
}

func writePodSpec(c *config.Config, state string, restore bool) {
	c.UpdateInstance()
	config.WritePodSpec(config.NewEtcdPod(c, state, false), c.PodSpecFile)
}

func addMember(c *config.Config) error {
	resp, err := etcdutilextra.AddMember(c.ClientURLs, c.LocalPeerURLs, c.TLSConfig)
	switch err {
	case nil:
		logrus.Infof("Added member node: %v (%v)", resp.Member.Name, resp.Member.ID)
		return nil
	case rpctypes.ErrMemberExist:
		logrus.Infof("Member already exists")
		return nil
	default:
		logrus.Errorf("Failed to add new member: %v", err)
		return err
	}
}

func removeMember(c *config.Config, memberID uint64) error {
	err := etcdutil.RemoveMember(c.ClientURLs, c.TLSConfig, memberID)
	switch err {
	case nil:
		logrus.Infof("Removed member: %v", memberID)
		return nil
	case rpctypes.ErrMemberNotFound:
		logrus.Infof("Member already removed")
		return nil
	default:
		logrus.Errorf("Failed to remove member (%v): %v", memberID, err)
		return err
	}
}
