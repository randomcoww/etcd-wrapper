package wrapper

import (
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"go.etcd.io/etcd/pkg/transport"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

type member struct {
	name string
	id *uint64
	peerURL string
	clientURL string
	healthy bool
	revision *int64
	clusterID *uint64
	leaderID *uint64
}

type Status struct {
	LeaderID *uint64
	ClusterID *uint64
	MemberNameMap map[string]*member
	MemberPeerMap map[string]*member
	MemberClientMap map[string]*member
	MemberSelf *member
	WritePodManifest func(string, bool, uint64) error
	ClientTLSConfig       *tls.Config
	healty bool
}

type config struct {
	ListenClientURLs []string
	ListenPeerURLs []string
	ClusterClientURLs []string
}

func newConfig() (*status, error) {
	status := &status{}
	var err error

	// etcd args
	var name, certFile, keyFile, trustedCAFile, peerCertFile, peerKeyFile, peerTrustedCAFile, initialAdvertisePeerURLs, listenPeerURLs, advertiseClientURLs, listenClientURLs, initialClusterToken, initialCluster string
	flag.StringVar(&name, "name", "", "Human-readable name for this member.")
	flag.StringVar(&certFile, "host-cert-file", "", "Host path to the client server TLS cert file.")
	flag.StringVar(&keyFile, "host-key-file", "", "Host path to the client server TLS key file.")
	flag.StringVar(&trustedCAFile, "host-trusted-ca-file", "", "Host path to the client server TLS trusted CA cert file.")
	flag.StringVar(&peerCertFile, "host-peer-cert-file", "", "Host path to the peer server TLS cert file.")
	flag.StringVar(&peerKeyFile, "host-peer-key-file", "", "Host path to the peer server TLS key file.")
	flag.StringVar(&peerTrustedCAFile, "host-peer-trusted-ca-file", "", "Host path to the peer server TLS trusted CA file.")
	flag.StringVar(&initialAdvertisePeerURLs, "initial-advertise-peer-urls", "", "List of this member's peer URLs to advertise to the rest of the cluster.")
	flag.StringVar(&listenPeerURLs, "listen-peer-urls", "", "List of URLs to listen on for peer traffic.")
	flag.StringVar(&advertiseClientURLs, "advertise-client-urls", "", "List of this member's client URLs to advertise to the public.")
	flag.StringVar(&listenClientURLs, "listen-client-urls", "", "List of URLs to listen on for client traffic.")
	flag.StringVar(&initialClusterToken, "initial-cluster-token", "", "Initial cluster token for the etcd cluster during bootstrap.")
	flag.StringVar(&initialCluster, "initial-cluster", "", "Initial cluster configuration for bootstrapping.")

	// pod manifest args
	var etcdPodName, etcdPodNamespace, etcdImage, snapRestoreFile, podManifestFile string
	flag.StringVar(&etcdPodName, "etcd-pod-name", "etcd", "Name of etcd pod.")
	flag.StringVar(&etcdPodNamespace, "etcd-pod-namespace", "kube-system", "Namespace to launch etcd pod.")
	flag.StringVar(&etcdImage, "etcd-image", "", "Etcd container image.")
	flag.StringVar(&snapRestoreFile, "host-snap-restore-file", "/var/lib/etcd-restore/etcd.db", "Host path to restore snapshot file.")
	flag.StringVar(&podManifestFile, "host-etcd-manifest-file", "", "Host path to write etcd pod manifest file. This should be where kubelet reads static pod manifests.")

	// etcd wrapper args
	var clientCertFile, clientKeyFile, clusterClientURLs, s3SnapBackupPath string
	var snapBackupInterval, healthCheckInterval, podManifestUpdateWait time.Duration
	var healthCheckFailuresAllow int
	flag.StringVar(&clientCertFile, "client-cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&clientKeyFile, "client-key-file", "", "Path to the client server TLS key file.")
	flag.StringVar(&initialClusterClients, "initial-cluster-clients", "", "List of etcd nodes and client URLs in same format as intial-cluster.")
	flag.StringVar(&s3SnapBackupPath, "s3-backup-path", "", "S3 key name for backup.")
	flag.DurationVar(&snapBackupInterval, "backup-interval", 30*time.Minute, "Backup trigger interval.")
	flag.DurationVar(&healthCheckInterval, "healthcheck-interval", 5*time.Second, "Healthcheck interval.")
	flag.IntVar(&healthCheckFailuresAllow, "healthcheck-failures-allow", 3, "Number of healthcheck failures to allow before updating etcd pod.")
	flag.DurationVar(&podManifestUpdateWait, "pod-update-wait", 30*time.Second, "Time to wait after pod manifest update to resume health checks.")
	flag.Parse()

	clusterClientURLsList := strings.Split(clusterClientURLs, ",")

	tlsInfo := transport.TLSInfo{
		CertFile:      clientCertFile,
		KeyFile:       clientKeyFile,
		TrustedCAFile: trustedCAFile,
	}
	status.ClientTLSConfig, err = tlsInfo.ClientConfig()
	if err != nil {
		return err
	}

	status.MemberNameMap = make(map[string]*member)
	status.MemberPeerMap = make(map[string]*member)
	status.MemberClientMap = make(map[string]*member)

	for _, n := range strings.Split(initialCluster, ",") {
		node = strings.Split(n, "=")
		m = &member{
			Name: node[0],
			PeerURL: node[1],
			Healthy: false,
		}
		status.MemberNameMap[m.Name] = m
		status.MemberPeerMap[m.PeerURL] = m
		if node[0] == name {
			status.MemberSelf = m
		}
	}

	for _, client := range strings.Split(initialClusterClients, ",") {
		node = strings.Split(n, "=")
		name := node[0]
		client := node[1]
		m, ok := status.MemberNameMap[name]
		if !ok {
			return nil, fmt.Errorf("Mismatch in initial-cluster and initial-cluster-clients members")
		}
		m.ClientURL = client
		status.MemberClientMap[client] = m
	}

	if len(status.MemberClientMap) != len(status.MemberNameMap) {
		return nil, fmt.Errorf("Mismatch in initial-cluster and initial-cluster-clients members")
	}

	if status.MemberSelf == nil {
		return fmt.Errorf("Member config not found for self (%s)", name)
	}

	status.WritePodManifest = func(initialClusterState string, snapRestore bool) {
		var id uint64
		if status.MemberSelf.id != nil {
			id = *status.MemberSelf.id
		}
		podspec.WriteManifest(
			name, certFile, keyFile, trustedCAFile, peerCertFile, peerKeyFile, peerTrustedCAFile, initialAdvertisePeerURLs,
			listenPeerURLs, advertiseClientURLs, listenClientURLs, initialClusterToken, initialCluster,
			etcdPodName, etcdPodNamespace, etcdImage, snapRestoreFile, podManifestFile,
			initialClusterState, snapRestore, id,
		)
		return nil
	}

	status.QueryMembers = func() (*clientv3.ListMembersResponse, error) {
		return etcdutil.ListMembers(clusterClientURLsList, v.ClientTLSConfig)
	}

	return status
}

func (v *status) UpdateFromHealthCheck() {
	err := etcdutil.Status(clusterClientURLsList, tlsConfig)
	v.Healthy = err == nil
}

func (v *status) UpdateFromStatus() error {
	respCh, err := etcdutil.Status(clusterClientURLsList, tlsConfig)
	var leaderID *uint64
	var count int

	for {
		resp := <-respCh
		count++

		m, ok := v.MemberClientMap[resp.endpoint]
		if !ok || m == nil {
			continue
		}

		if resp.err != nil {
			m.Healthy = false
			continue
		}

		m.Healthy = true
		m.MemberID = resp.status.ResponseHeader.MemberID
		m.ClusterID = resp.status.ResponseHeader.ClusterID
		m.LeaderID = resp.status.Leader
		m.Revision = resp.status.ResponseHeader.Revision

		if count >= len(clusterClientURLsList) {
			close(respCh)
			return nil
		}
	}
}

func (v *status) UpdateFromMembers() error {
	members, err := v.QueryMembers()
	if err != nil {
		return err
	}

	// member Name field may not be populated right away
	// Match returned members by PeerURL field
	peerURLsReturned := make(map[string]struct{})
	for _, member := range members.Members {
		var m *member
		var ok bool

		for _, peer := range member.PeerURLs {
			var id uint64
			if m, ok = v.MemberPeerMap[peer]; ok {
				id = member.ID
				m.ID = &id

				peerURLsReturned[peer] = struct{}{}
				break
			}
		}
	}

	// Compare returned members with list and remove inactive ones
	for peer, m := range v.MemberPeerMap {
		if _, ok := peerURLsReturned[peer]; !ok	{
			m.ID = nil
		}
	}
}

func (v *config) AddMemberSelf() error {
	if v.MemberSelf.id != nil {
		return nil
	}
	resp, err := etcdutil.AddMember(v.ListenClientURLs, v.ListenPeerURLs, v.ClientTLSConfig)
	if err != nil {
		return err
	}
	id = resp.Member.ID
	v.MemberSelf.id = &id
	return nil
}

func (v *config) RemoveMemberSelf() error {
	if v.MemberSelf.id == nil {
		return nil
	}
	err := etcdutil.RemoveMember(v.ListenClientURLs, v.ClientTLSConfig, *v.MemberSelf.id)
	if err != nil {
		return err
	}
	v.MemberSelf.id = nil
	return nil
}