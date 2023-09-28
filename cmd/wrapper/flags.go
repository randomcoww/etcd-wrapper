package wrapper

import (
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/pkg/transport"
	"os"
	"time"
)

type Member struct {
	Name      string
	PeerURL   string
	ClientURL string
	Revision  *int64
	MemberID        *uint64
	ClusterID *uint64
	LeaderID  *uint64
}

type Status struct {
	ClusterID        *uint64
	Revision        *uint64
	MemberMap    map[string]*Member
	MemberPeerMap    map[string]*Member
	MemberClientMap  map[string]*Member
	MemberSelf       *Member
	ClientTLSConfig  *tls.Config
	WritePodManifest func(string, bool, uint64) error
}

func newStatus() (*Status, error) {
	status := &Status{}
	var err error

	// etcd args
	var name, certFile, keyFile, trustedCAFile, peerCertFile, peerKeyFile, peerTrustedCAFile, initialAdvertisePeerURLs, listenPeerURLs, advertiseClientURLs, listenClientURLs, initialClusterToken, initialCluster string
	flag.StringVar(&name, "name", "", "Human-readable name for this member.")
	flag.StringVar(&certFile, "cert-file", "", "Host path to the client server TLS cert file.")
	flag.StringVar(&keyFile, "key-file", "", "Host path to the client server TLS key file.")
	flag.StringVar(&trustedCAFile, "trusted-ca-file", "", "Host path to the client server TLS trusted CA cert file.")
	flag.StringVar(&peerCertFile, "peer-cert-file", "", "Host path to the peer server TLS cert file.")
	flag.StringVar(&peerKeyFile, "peer-key-file", "", "Host path to the peer server TLS key file.")
	flag.StringVar(&peerTrustedCAFile, "peer-trusted-ca-file", "", "Host path to the peer server TLS trusted CA file.")
	flag.StringVar(&initialAdvertisePeerURLs, "initial-advertise-peer-urls", "", "List of this member's peer URLs to advertise to the rest of the cluster.")
	flag.StringVar(&listenPeerURLs, "listen-peer-urls", "", "List of URLs to listen on for peer traffic.")
	flag.StringVar(&advertiseClientURLs, "advertise-client-urls", "", "List of this member's client URLs to advertise to the public.")
	flag.StringVar(&listenClientURLs, "listen-client-urls", "", "List of URLs to listen on for client traffic.")
	flag.StringVar(&initialClusterToken, "initial-cluster-token", "", "Initial cluster token for the etcd cluster during bootstrap.")
	flag.StringVar(&initialCluster, "initial-cluster", "", "Initial cluster configuration for bootstrapping.")

	// pod manifest args
	var etcdImage, etcdPodName, etcdPodNamespace, etcdSnapshotPath, etcdPodManifestPath string
	flag.StringVar(&etcdImage, "etcd-image", "", "Etcd container image.")
	flag.StringVar(&etcdPodName, "etcd-pod-name", "etcd", "Name of etcd pod.")
	flag.StringVar(&etcdPodNamespace, "etcd-pod-namespace", "kube-system", "Namespace to launch etcd pod.")
	flag.StringVar(&etcdSnapshotFile, "etcd-snaphot-file", "/var/lib/etcd/etcd.db", "Host path to restore snapshot file.")
	flag.StringVar(&etcdPodManifestFile, "etcd-pod-manifest-file", "", "Host path to write etcd pod manifest file. This should be where kubelet reads static pod manifests.")

	// etcd wrapper args
	var clientCertFile, clientKeyFile, initialClusterClients, s3BackupResource string
	var snapBackupInterval, healthCheckInterval, podManifestUpdateWait time.Duration
	var healthCheckFailuresAllow int
	flag.StringVar(&clientCertFile, "client-cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&clientKeyFile, "client-key-file", "", "Path to the client server TLS key file.")
	flag.StringVar(&initialClusterClients, "initial-cluster-clients", "", "List of etcd nodes and client URLs in same format as intial-cluster.")
	flag.StringVar(&s3BackupResource, "s3-backup-resource", "", "S3 resource name for backup.")
	flag.DurationVar(&snapBackupInterval, "backup-interval", 30*time.Minute, "Backup trigger interval.")
	flag.DurationVar(&healthCheckInterval, "healthcheck-interval", 5*time.Second, "Healthcheck interval.")
	flag.IntVar(&healthCheckFailuresAllow, "healthcheck-failures-allow", 3, "Number of healthcheck failures to allow before updating etcd pod.")
	flag.DurationVar(&podManifestUpdateWait, "pod-update-wait", 30*time.Second, "Time to wait after pod manifest update to resume health checks.")
	flag.Parse()

	tlsInfo := transport.TLSInfo{
		CertFile:      clientCertFile,
		KeyFile:       clientKeyFile,
		TrustedCAFile: trustedCAFile,
	}
	status.ClientTLSConfig, err = tlsInfo.ClientConfig()
	if err != nil {
		return err
	}

	status.MemberMap = make(map[string]*Member)
	status.MemberPeerMap = make(map[string]*Member)
	status.MemberClientMap = make(map[string]*Member)

	for _, n := range strings.Split(initialCluster, ",") {
		node = strings.Split(n, "=")
		m = &member{
			Name:    node[0],
			PeerURL: node[1],
			Healthy: false,
		}
		status.MemberMap[m.Name] = m
		status.MemberPeerMap[m.PeerURL] = m
		if node[0] == name {
			status.MemberSelf = m
		}
	}

	for _, client := range strings.Split(initialClusterClients, ",") {
		node = strings.Split(n, "=")
		name := node[0]
		client := node[1]
		m, ok := status.MemberMap[name]
		if !ok {
			return nil, fmt.Errorf("Mismatch in initial-cluster and initial-cluster-clients members")
		}
		m.ClientURL = client
		status.MemberClientMap[client] = m
	}

	if len(status.MemberClientMap) != len(status.MemberMap) {
		return nil, fmt.Errorf("Mismatch in initial-cluster and initial-cluster-clients members")
	}

	if status.MemberSelf == nil {
		return fmt.Errorf("Member config not found for self (%s)", name)
	}

	status.WritePodManifest = func(initialClusterState string, runRestore bool) {
		var id uint64
		if status.MemberSelf.id != nil {
			id = *status.MemberSelf.id
		}
		podspec.WriteManifest(
			name, certFile, keyFile, trustedCAFile, peerCertFile, peerKeyFile, peerTrustedCAFile, initialAdvertisePeerURLs,
			listenPeerURLs, advertiseClientURLs, listenClientURLs, initialClusterToken, initialCluster,
			etcdImage, etcdPodName, etcdPodNamespace, etcdSnapshotFile, etcdPodManifestFile,
			initialClusterState, runRestore, id,
		)
		return nil
	}
	return status
}