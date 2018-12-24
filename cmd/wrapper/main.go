package wrapper

import (
	"crypto/tls"
	"io/ioutil"
	"time"

	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/backup"
	"github.com/randomcoww/etcd-wrapper/pkg/cluster"
	"github.com/randomcoww/etcd-wrapper/pkg/podutil"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
)

const (
	StateCheckCluster      = 0
	StateCheckBackup       = 1
	StateStartNew          = 2
	StateStartWithSnapshot = 3
	StateStartExisting     = 4
	StateWaitPodSpec       = 5
	StateCheckMembers      = 6
	StateRemoveLocal       = 7
	StateAddLocal          = 8
)

func Main() {
	config := cluster.NewCluster()
	clientURLs := cluster.ClientURLsFromConfig(config)

	cert, _ := ioutil.ReadFile(config.CertFile)
	key, _ := ioutil.ReadFile(config.KeyFile)
	ca, _ := ioutil.ReadFile(config.TrustedCAFile)
	tlsConfig, err := etcdutil.NewTLSConfig(cert, key, ca)
	if err != nil {
		logrus.Errorf("Failed to read TLS files: %v", err)
		return
	}

	go runBackupHandler(config, clientURLs, tlsConfig)
	run(config, clientURLs, tlsConfig)
}

func run(c *cluster.Cluster, clientURLs []string, tlsConfig *tls.Config) {
	logrus.Infof("Start etcd-wrapper for clients: %v", clientURLs)

	localClientULRs := cluster.LocalClientURLsFromConfig(c)
	localPeerURLs := cluster.LocalPeerURLsFromConfig(c)

	// memberset from config
	memberSet := cluster.MemberSet{}
	for _, memberName := range cluster.MemberURLsFromConfig(c) {
		member := cluster.NewMember(memberName)
		memberSet[memberName] = member
	}

	// initial state to start
	state := StateCheckCluster

	for {
		switch state {
		// check response from cluster
		case StateCheckCluster:
			logrus.Infof("Check cluster")
			memberList, err := etcdutil.ListMembers(clientURLs, tlsConfig)
			if err != nil {
				// Update annotation to restart pod
				logrus.Errorf("Could not get member list")
				c.UpdateInstance()
				state = StateCheckBackup
			} else {
				// Fill in member ID from etcd
				for _, member := range memberList.Members {
					logrus.Infof("Found member: %v (%v)", member.Name, member.ID)
					memberSet[member.Name].ID = member.ID
				}
				state = StateCheckMembers
			}
			// check backup
		case StateCheckBackup:
			logrus.Infof("Check backup")
			err := backup.FetchBackup(c.S3BackupPath, c.BackupFile)
			if err != nil {
				logrus.Errorf("Could not download snapshot backup: %v", err)
				state = StateStartNew
			} else {
				logrus.Infof("Found snapshot backup")
				state = StateStartWithSnapshot
			}
			// no backup - new podspec
		case StateStartNew:
			logrus.Infof("Start pod spec new")
			podutil.WritePodSpec(podutil.NewEtcdPod(c, "new", false), c.PodSpecFile)
			state = StateWaitPodSpec
			// start with backup
		case StateStartWithSnapshot:
			logrus.Infof("Start pod spec with snapshot")
			podutil.WritePodSpec(podutil.NewEtcdPod(c, "existing", true), c.PodSpecFile)
			state = StateWaitPodSpec
			// state in existing state
		case StateStartExisting:
			logrus.Infof("Start pod spec with existing")
			podutil.WritePodSpec(podutil.NewEtcdPod(c, "existing", false), c.PodSpecFile)
			state = StateWaitPodSpec
			// wait after podspec
		case StateWaitPodSpec:
			logrus.Infof("Wait pod spec")
			time.Sleep(c.PodSpecWait)
			state = StateCheckCluster
			// check each member status for unresponsive
		case StateCheckMembers:
			logrus.Infof("Check members")
			// Check status of local member
			status, err := etcdutilextra.Status(localClientULRs, tlsConfig)
			if err != nil {
				// Set local ID
				logrus.Errorf("Healthcheck failed: %v", c.Name)
				state = StateRemoveLocal
			} else {
				c.ID = memberSet[c.Name].ID
				if len(status.Errors) == 0 {
					logrus.Infof("Healthcheck success: %v (%v)", c.Name, c.ID)
					time.Sleep(c.RunInterval)
				} else {
					logrus.Errorf("Status error: %v", status.Errors)
					state = StateRemoveLocal
				}
			}
			// Remove local node
		case StateRemoveLocal:
			logrus.Infof("Remove local")
			err := etcdutil.RemoveMember(clientURLs, tlsConfig, c.ID)
			switch err {
			case nil, rpctypes.ErrMemberNotFound:
				logrus.Infof("Removed local node")
				// state = StateStartWithSnapshot
				state = StateAddLocal
			default:
				logrus.Errorf("Failed to remove local node: %v", err)
				time.Sleep(c.MemberWait)
			}
			// Add local node
		case StateAddLocal:
			logrus.Infof("Add local")
			resp, err := etcdutilextra.AddMember(clientURLs, localPeerURLs, tlsConfig)
			switch err {
			case nil:
				memberSet[resp.Member.Name].ID = resp.Member.ID
				c.ID = resp.Member.ID
				logrus.Infof("Added member node: %v (%v)", resp.Member.Name, resp.Member.ID)
				state = StateStartExisting
			case rpctypes.ErrMemberExist:
				logrus.Infof("Member exists")
				state = StateStartExisting
			default:
				logrus.Errorf("Failed to add new member: %v", err)
				time.Sleep(c.MemberWait)
			}
		}
	}
}