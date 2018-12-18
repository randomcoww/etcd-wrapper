package wrapper

import (
	"context"
	"crypto/tls"
	"flag"
	"io/ioutil"
	"time"

	"github.com/randomcoww/etcd-wrapper/pkg/podutil"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/coreos/etcd-operator/pkg/backup"
	"github.com/coreos/etcd-operator/pkg/backup/reader"
	"github.com/coreos/etcd-operator/pkg/backup/writer"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/restore"
	etcdutilext "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"

	"github.com/coreos/etcd-operator/pkg/util/constants"
	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
)

var (
	Member *podutil.Spec
)

func parseFlags() {
	Member = new(podutil.Spec)

	flag.StringVar(&Member.Name, "name", "", "Human-readable name for this member.")

	flag.StringVar(&Member.BackupMountDir, "backup-dir", "/var/lib/etcd-restore", "Base path of snapshot restore file.")
	flag.StringVar(&Member.BackupFile, "backup-file", "/var/lib/etcd-restore/etcd.db", "Snapshot file restore path.")

	flag.StringVar(&Member.EtcdTLSMountDir, "tls-dir", "/etc/ssl/cert", "Base path of TLS cert files.")
	flag.StringVar(&Member.CertFile, "cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&Member.KeyFile, "key-file", "", "Path to the client server TLS key file.")
	flag.StringVar(&Member.TrustedCAFile, "trusted-ca-file", "", "Path to the client server TLS trusted CA cert file.")
	flag.StringVar(&Member.PeerCertFile, "peer-cert-file", "", "Path to the peer server TLS cert file.")
	flag.StringVar(&Member.PeerKeyFile, "peer-key-file", "", "Path to the peer server TLS key file.")
	flag.StringVar(&Member.PeerTrustedCAFile, "peer-trusted-ca-file", "", "Path to the peer server TLS trusted CA file.")

	flag.StringVar(&Member.InitialAdvertisePeerURLs, "initial-advertise-peer-urls", "", "List of this member's peer URLs to advertise to the rest of the cluster.")
	flag.StringVar(&Member.ListenPeerURLs, "listen-peer-urls", "", "List of URLs to listen on for peer traffic.")

	flag.StringVar(&Member.AdvertiseClientURLs, "advertise-client-urls", "", "List of this member's client URLs to advertise to the public.")
	flag.StringVar(&Member.ListenClientURLs, "listen-client-urls", "", "List of URLs to listen on for client traffic.")

	flag.StringVar(&Member.InitialClusterToken, "initial-cluster-token", "", "Initial cluster token for the etcd cluster during bootstrap.")
	flag.StringVar(&Member.InitialCluster, "initial-cluster", "", "Initial cluster configuration for bootstrapping.")

	flag.StringVar(&Member.Image, "image", "quay.io/coreos/etcd:v3.3", "Etcd container image.")
	flag.StringVar(&Member.PodSpecFile, "pod-spec-file", "", "Pod spec file path (intended to be in kubelet manifests path).")
	flag.StringVar(&Member.S3BackupPath, "s3-backup-path", "", "S3 key name for backup.")
	flag.Parse()
}

func Main() {
	parseFlags()

	cert, _ := ioutil.ReadFile(Member.CertFile)
	key, _ := ioutil.ReadFile(Member.KeyFile)
	ca, _ := ioutil.ReadFile(Member.TrustedCAFile)

	tlsConfig, err := etcdutil.NewTLSConfig(cert, key, ca)
	if err != nil {
		logrus.Errorf("Failed to read TLS file: %v", err)
		return
	}

	logrus.Infof("Start")

	go runMain(podutil.ClientURLs(Member), tlsConfig)
	runBackup(podutil.ClientURLs(Member), tlsConfig)
}

func runBackup(clientURLs []string, tlsConfig *tls.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)

	for {
		select {
		case <-time.After(30 * time.Second):
			logrus.Infof("Started backup")

			status, err := etcdutilext.Status(clientURLs, tlsConfig)
			if err != nil {
				logrus.Errorf("Failed to get cluster status: %v", err)
				continue
			}

			if len(status.Errors) > 0 {
				logrus.Errorf("Errors found in cluster status: %v", status.Errors)
				continue
			}

			// Check backup
			sess := session.Must(session.NewSession(&aws.Config{
				// Region: aws.String("us-west-2"),
			}))

			s3Writer := writer.NewS3Writer(s3.New(sess))
			bm := backup.NewBackupManagerFromWriter(nil, s3Writer, tlsConfig, clientURLs, "")
			_, _, err = bm.SaveSnap(ctx, Member.S3BackupPath)
			cancel()

			if err != nil {
				logrus.Errorf("Failed to run backup: %v", err)
				continue
			}

			logrus.Infof("Finished backup")
		}
	}
}

func runMain(clientURLs []string, tlsConfig *tls.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)

	for {
		select {
		case <-time.After(10 * time.Second):
			// Check member list
			memberList, err := etcdutil.ListMembers(clientURLs, tlsConfig)
			if err != nil {
				logrus.Errorf("Failed to get etcd member list: %v", err)

				// Check backup
				sess := session.Must(session.NewSession(&aws.Config{
					// Region: aws.String("us-west-2"),
				}))

				s3Reader := reader.NewS3Reader(s3.New(sess))
				rm := restore.NewRestoreManagerFromReader(s3Reader)
				err = rm.DownloadSnap(ctx, Member.S3BackupPath, Member.BackupFile)
				cancel()

				if err != nil {
					logrus.Errorf("Failed to download backup: %v", err)

					// Write pod spec for new cluster
					err = podutil.WritePodSpec(podutil.NewEtcdPod(Member, "new", false), Member.PodSpecFile)
					if err != nil {
						logrus.Errorf("Failed to create pod spec: %v", err)
						continue
					}
					logrus.Infof("Created pod spec: %v", Member.PodSpecFile)
					continue
				}

				// Start existing if backup exists
				// Write pod spec for existing cluster with restore
				err = podutil.WritePodSpec(podutil.NewEtcdPod(Member, "existing", true), Member.PodSpecFile)
				if err != nil {
					logrus.Errorf("Failed to create pod spec: %v", err)
				}
				continue
			}

			for _, member := range memberList.Members {
				// logrus.Infof("Got member: %v (%v)", member.ID, member.Name)
				_, err := etcdutil.ListMembers(member.ClientURLs, tlsConfig)

				if err != nil {
					// Remove this node
					err = etcdutil.RemoveMember(clientURLs, tlsConfig, member.ID)
					switch err {
					case nil:
						logrus.Infof("Removed member: %v (%v)", member.ID, member.Name)
					case rpctypes.ErrMemberNotFound:
						logrus.Infof("Member already removed: %v (%v)", member.ID, member.Name)
					default:
						logrus.Errorf("Failed to remove member: %v (%v)", member.ID, member.Name)
						break
					}

					// If this is my node
					if member.Name == Member.Name {
						// Replace with a new member that will be used once new pod starts
						err = etcdutilext.AddMember(clientURLs, member.PeerURLs, tlsConfig)
						if err != nil {
							logrus.Errorf("Failed to add member: %v", err)
							break
						}
						logrus.Infof("Added new member")

						// New write pod spec to start new member in "existing" state
						err = podutil.WritePodSpec(podutil.NewEtcdPod(Member, "existing", false), Member.PodSpecFile)
						if err != nil {
							logrus.Errorf("Failed to create pod spec: %v", err)
							break
						}
					}
				}
				logrus.Infof("Node active: %v (%v)", member.ID, member.Name)
				break
			}
		}
	}
}
