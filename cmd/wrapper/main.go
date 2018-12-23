package wrapper

import (
	"crypto/tls"
	"io/ioutil"
	"time"

	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/sirupsen/logrus"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/cluster"
	"github.com/randomcoww/etcd-wrapper/pkg/podutil"
	"github.com/randomcoww/etcd-wrapper/pkg/s3backup"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
)

var (
	Config *cluster.Cluster
)

func Main() {
	Config = cluster.NewCluster()
	clientURLs := cluster.ClientURLsFromConfig(Config)

	cert, _ := ioutil.ReadFile(Config.CertFile)
	key, _ := ioutil.ReadFile(Config.KeyFile)
	ca, _ := ioutil.ReadFile(Config.TrustedCAFile)
	tlsConfig, err := etcdutil.NewTLSConfig(cert, key, ca)
	if err != nil {
		logrus.Errorf("Failed to read TLS files: %v", err)
		return
	}
	logrus.Infof("Start etcd-wrapper for clients: %v", clientURLs)

	// Start with member initialization
	Config.TriggerRestore()

	// Start backup handler
	go runBackup(clientURLs, tlsConfig)

	run(clientURLs, tlsConfig)
}

func run(clientURLs []string, tlsConfig *tls.Config) {
	logrus.Infof("Start etcd-wrapper for clients: %v", clientURLs)

	reaperSet := cluster.ReaperSet{}
	configMemberSet := cluster.NewMemberSetFromConfig(Config)
	listenPeerURLs := cluster.ListenPeerURLsFromConfig(Config)

	for {
		select {
		case <-Config.RunRestore:
			logrus.Errorf("Restore etcd member")
			// Restarts pod as needed
			Config.UpdateInstance()

			err := s3backup.FetchBackup(Config.S3BackupPath, Config.BackupFile)
			if err != nil {
				logrus.Infof("Could not download snapshot backup: %v", err)
				// Downloaded backup data
				// Write pod spec for existing cluster with restore
				podutil.WritePodSpec(podutil.NewEtcdPod(Config, "new", false), Config.PodSpecFile)
				logrus.Infof("Generated etcd pod spec for new cluster")

			} else {
				logrus.Infof("Found snapshot backup")
				// Start new cluster
				podutil.WritePodSpec(podutil.NewEtcdPod(Config, "existing", true), Config.PodSpecFile)
				logrus.Errorf("Generated etcd pod spec for existing cluster")
			}
			logrus.Infof("Restore handler sleep for %v", Config.RestoreInterval)
			time.Sleep(Config.RestoreInterval)

		case <-time.After(Config.RunInterval):
			// Check member list
			memberList, err := etcdutil.ListMembers(clientURLs, tlsConfig)
			if err != nil {
				logrus.Errorf("Failed to get etcd member list: %v", err)
				Config.TriggerRestore()
				continue
			}

			// Cluster is healthy
			// Create pod spec if my node is missing
			for _, member := range configMemberSet.Diff(cluster.NewMemberSetFromList(memberList)) {
				logrus.Infof("Member missing: %v", member.Name)

				if Config.Name == member.Name {
					// My node is missing from etcd list members
					// Send add member request
					logrus.Infof("Add missing member: %v %v", member.Name, listenPeerURLs)

					// err = etcdutilextra.AddMember(clientURLs, peerURLs, tlsConfig)
					err = etcdutilextra.AddMember(clientURLs, listenPeerURLs, tlsConfig)
					if err != nil {
						logrus.Errorf("Failed to add new member: %v (%v)", member.Name, err)
					}

					// Create a pod spec to add to existing cluster
					podutil.WritePodSpec(podutil.NewEtcdPod(Config, "existing", false), Config.PodSpecFile)
					logrus.Infof("Wrote etcd pod spec for existing cluster: %v", member.Name)
					break
				}
			}

			// Hit each member and remove nodes that don't respond
			for _, member := range memberList.Members {
				// Remove nodes that don't respond for 1 minute
				r, ok := reaperSet[member.Name]
				if !ok {
					reaperSet[member.Name] = &cluster.Reaper{
						Reset: make(chan struct{}, 1),
						Fn: func(memberName string, memberID uint64, isMyNode bool) {
							logrus.Infof("Start reaper for member: %v (%v)", memberName, memberID)
							for {
								select {
								case <-time.After(Config.EtcdTimeout):
									logrus.Errorf("Member unresponsive: %v (%v)", memberName, memberID)

									err = etcdutil.RemoveMember(clientURLs, tlsConfig, memberID)
									switch err {
									case nil:
										logrus.Infof("Removed member: %v (%v)", memberName, memberID)
									case rpctypes.ErrMemberNotFound:
										logrus.Infof("Member already removed: %v (%v)", memberName, memberID)
									default:
										logrus.Errorf("Failed to remove member: %v (%v)", memberName, err)
										if isMyNode {
											Config.TriggerRestore()
										}
									}
								case <-r.Reset:
								}
							}
						},
					}
					r = reaperSet[member.Name]
					go r.Fn(member.Name, member.ID, Config.Name == member.Name)
				}

				// Test getting status from URL of just this member
				status, err := etcdutilextra.Status(member.ClientURLs, tlsConfig)
				if err != nil {
					logrus.Errorf("Healthcheck failed: %v %v", member.ID, member.Name)
				} else {
					// logrus.Infof("Healthcheck success: %v %v", member.ID, member.Name)
					r.ResetTimeout()

					// Check if this node is leader. Run backup if leader.
					if Config.Name == member.Name && status.Leader == member.ID {
						Config.TriggerBackup()
					}
				}
			}
		}
	}
}

func runBackup(clientURLs []string, tlsConfig *tls.Config) {
	logrus.Infof("Start backup handler for clients")
	for {
		select {
		case <-Config.RunBackup:
			logrus.Infof("Start backup process")
			err := s3backup.SendBackup(Config.S3BackupPath, tlsConfig, clientURLs)

			if err != nil {
				logrus.Errorf("Backup failed: %v", err)
			} else {
				logrus.Infof("Finished backup")
			}
			logrus.Infof("Backup handler sleep for %v", Config.BackupInterval)
			time.Sleep(Config.BackupInterval)
		}
	}
}
