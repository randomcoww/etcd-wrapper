package wrapper

import (
	"context"
	"crypto/tls"
	"flag"
	"io/ioutil"
	"time"
	"strconv"

	"github.com/randomcoww/etcd-wrapper/pkg/podutil"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/coreos/etcd-operator/pkg/backup"
	"github.com/coreos/etcd-operator/pkg/backup/reader"
	"github.com/coreos/etcd-operator/pkg/backup/writer"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/cluster"
	"github.com/randomcoww/etcd-wrapper/pkg/restore"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"

	"github.com/coreos/etcd-operator/pkg/util/constants"
	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
)

var (
	Config *cluster.Cluster
)

func parseFlags() {
	Config = new(cluster.Cluster)
	flag.StringVar(&Config.Name, "name", "", "Human-readable name for this member.")

	flag.StringVar(&Config.EtcdServers, "etcd-servers", "", "List of etcd client URLs.")
	flag.StringVar(&Config.Image, "image", "quay.io/coreos/etcd:v3.3", "Etcd container image.")
	flag.StringVar(&Config.PodSpecFile, "pod-spec-file", "", "Pod spec file path (intended to be in kubelet manifests path).")
	// Backup
	flag.StringVar(&Config.S3BackupPath, "s3-backup-path", "", "S3 key name for backup.")
	flag.StringVar(&Config.BackupMountDir, "backup-dir", "/var/lib/etcd-restore", "Base path of snapshot restore file.")
	flag.StringVar(&Config.BackupFile, "backup-file", "/var/lib/etcd-restore/etcd.db", "Snapshot file restore path.")
	// TLS
	flag.StringVar(&Config.EtcdTLSMountDir, "tls-dir", "/etc/ssl/cert", "Base path of TLS cert files.")
	flag.StringVar(&Config.CertFile, "cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&Config.KeyFile, "key-file", "", "Path to the client server TLS key file.")
	flag.StringVar(&Config.TrustedCAFile, "trusted-ca-file", "", "Path to the client server TLS trusted CA cert file.")
	flag.StringVar(&Config.PeerCertFile, "peer-cert-file", "", "Path to the peer server TLS cert file.")
	flag.StringVar(&Config.PeerKeyFile, "peer-key-file", "", "Path to the peer server TLS key file.")
	flag.StringVar(&Config.PeerTrustedCAFile, "peer-trusted-ca-file", "", "Path to the peer server TLS trusted CA file.")
	// Pass to etcd
	flag.StringVar(&Config.InitialAdvertisePeerURLs, "initial-advertise-peer-urls", "", "List of this member's peer URLs to advertise to the rest of the cluster.")
	flag.StringVar(&Config.ListenPeerURLs, "listen-peer-urls", "", "List of URLs to listen on for peer traffic.")
	flag.StringVar(&Config.AdvertiseClientURLs, "advertise-client-urls", "", "List of this member's client URLs to advertise to the public.")
	flag.StringVar(&Config.ListenClientURLs, "listen-client-urls", "", "List of URLs to listen on for client traffic.")
	flag.StringVar(&Config.InitialClusterToken, "initial-cluster-token", "", "Initial cluster token for the etcd cluster during bootstrap.")
	flag.StringVar(&Config.InitialCluster, "initial-cluster", "", "Initial cluster configuration for bootstrapping.")
	// Check intervals
	flag.DurationVar(&Config.RunInterval, "run-interval", 10, "Member check interval.")
	flag.DurationVar(&Config.BackupInterval, "backup-interval", 300, "Member check interval.")
	flag.DurationVar(&Config.EtcdTimeout, "etcd-timeout", 120, "Etcd status check timeout.")

	flag.Parse()
}

func Main() {
	parseFlags()

	clientURLs := cluster.ClientURLsFromConfig(Config)

	cert, _ := ioutil.ReadFile(Config.CertFile)
	key, _ := ioutil.ReadFile(Config.KeyFile)
	ca, _ := ioutil.ReadFile(Config.TrustedCAFile)
	tlsConfig, err := etcdutil.NewTLSConfig(cert, key, ca)
	if err != nil {
		logrus.Errorf("Failed to read TLS files: %v", err)
		return
	}

	// Add label to pod spec to regenerate when this restarts
	Config.Instance = strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	Config.RunBackup = make(chan struct{}, 1)

	logrus.Infof("Start etcd-wrapper for clients: %v", clientURLs)

	go runBackup(clientURLs, tlsConfig)
	runMain(clientURLs, tlsConfig)
}

func runMain(clientURLs []string, tlsConfig *tls.Config) {
	logrus.Infof("Start etcd-wrapper for clients: %v", clientURLs)

	reaperSet := cluster.ReaperSet{}
	configMemberSet := cluster.NewMemberSetFromConfig(Config)
	listenPeerURLs := cluster.ListenPeerURLsFromConfig(Config)

	for {
		select {
		case <-time.After(Config.RunInterval):
			// Check member list
			memberList, err := etcdutil.ListMembers(clientURLs, tlsConfig)
			if err != nil {
				logrus.Errorf("Failed to get etcd member list: %v", err)

				// Check backup
				sess := session.Must(session.NewSession(&aws.Config{
					// Region: aws.String("us-west-2"),
				}))

				ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
				s3Reader := reader.NewS3Reader(s3.New(sess))
				rm := restore.NewRestoreManagerFromReader(s3Reader)
				err = rm.DownloadSnap(ctx, Config.S3BackupPath, Config.BackupFile)

				if err != nil {
					logrus.Infof("Could not download snapshot backup: %v", err)
					// Downloaded backup data
					// Write pod spec for existing cluster with restore

					podutil.WritePodSpec(podutil.NewEtcdPod(Config, "new", false), Config.PodSpecFile)
					logrus.Infof("Generated etcd pod spec for new cluster")
					continue

				} else {
					logrus.Infof("Found snapshot backup")
					// Start new cluster
					podutil.WritePodSpec(podutil.NewEtcdPod(Config, "existing", true), Config.PodSpecFile)
					logrus.Errorf("Generated etcd pod spec for existing cluster")
					continue
				}
				cancel()
			}

			// Create pod if my node is missing
			for _, member := range configMemberSet.Diff(cluster.NewMemberSetFromList(memberList)) {
				logrus.Infof("Member missing: %v", member.Name)

				if Config.Name == member.Name {
					// My node is missing from etcd list members
					// Send add member request
					logrus.Infof("Add missing member: %v (%v)", member.Name, listenPeerURLs)

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
						Fn: func(memberName string, memberID uint64) {
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
									}
								case <-r.Reset:
								}
							}
						},
					}
					r = reaperSet[member.Name]
					go r.Fn(member.Name, member.ID)
				}

				// Test getting status from URL of just this member
				status, err := etcdutilextra.Status(member.ClientURLs, tlsConfig)
				if err != nil {
					logrus.Errorf("Health check failed: %v (%v)", member.Name, member.ID)
				} else {
					logrus.Infof("Health check success: %v (%v)", member.Name, member.ID)

					// Reset reap timer
					select {
					case r.Reset <- struct{}{}:
					default:
					}

					// Check if this node is leader. Run backup if leader.
					if Config.Name == member.Name && status.Leader == member.ID {
						select {
						case Config.RunBackup <- struct{}{}:
						default:
						}
					}
				}
			}
		}
	}
}

func runBackup(clientURLs []string, tlsConfig *tls.Config) {
	logrus.Infof("Start backup process for clients: %v", clientURLs)
	for {
		select {
		case <-Config.RunBackup:
			logrus.Infof("Start backup from leader")
			// Check backup
			sess := session.Must(session.NewSession(&aws.Config{
				// Region: aws.String("us-west-2"),
			}))

			ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
			s3Writer := writer.NewS3Writer(s3.New(sess))
			bm := backup.NewBackupManagerFromWriter(nil, s3Writer, tlsConfig, clientURLs, "")
			_, _, err := bm.SaveSnap(ctx, Config.S3BackupPath)

			if err != nil {
				logrus.Errorf("Backup failed: %v", err)
			} else {
				logrus.Infof("Finished backup. Sleep for %v", Config.BackupInterval)
			}
			cancel()
			time.Sleep(Config.BackupInterval)
		}
	}
}
