package wrapper

import (
	"os"

	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/sirupsen/logrus"
)

func Main() {
	c, err := config.NewConfig()
	if err != nil {
		logrus.Errorf("Parse config failed: %v", err)
		os.Exit(1)
	}

	healthcheck := newHealthCheck(c)
	backup := newBackup(c)

	go healthcheck.runPeriodic()
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
