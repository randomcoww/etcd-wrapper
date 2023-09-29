package status

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/randomcoww/etcd-wrapper/pkg/podspec"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/util/s3util"
	"go.etcd.io/etcd/pkg/transport"
	"io"
	"k8s.io/api/core/v1"
	"strings"
	"time"
)

type Member struct {
	Name                string
	PeerURL             string
	ClientURL           string
	MemberID            *uint64
	MemberIDFromCluster *uint64
	Revision            *int64
	ClusterID           *uint64
	LeaderID            *uint64
}

type Status struct {
	ClusterID       *uint64
	LeaderID        *uint64
	BackupMemberID  *uint64
	Revision        *int64
	MemberMap       map[string]*Member
	MemberPeerMap   map[string]*Member
	MemberClientMap map[string]*Member
	MemberSelf      *Member
	Members         []*Member
	MembersHealthy  []*Member
	Healthy         bool
	ClientTLSConfig *tls.Config
	PodSpec         func(string, bool) *v1.Pod
	//
	S3BackupResource    string
	EtcdSnapshotFile    string
	EtcdPodManifestFile string
	ListenPeerURLs      []string
}

func New() (*Status, error) {
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
	var etcdImage, etcdPodName, etcdPodNamespace, etcdSnapshotFile, etcdPodManifestFile string
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
		return nil, err
	}

	status.S3BackupResource = s3BackupResource
	status.EtcdSnapshotFile = etcdSnapshotFile
	status.EtcdPodManifestFile = etcdPodManifestFile

	status.MemberMap = make(map[string]*Member)
	status.MemberPeerMap = make(map[string]*Member)
	status.MemberClientMap = make(map[string]*Member)

	status.ListenPeerURLs = strings.Split(listenPeerURLs, ",")

	for _, n := range strings.Split(initialCluster, ",") {
		node := strings.Split(n, "=")
		m := &Member{
			Name:    node[0],
			PeerURL: node[1],
		}
		status.MemberMap[m.Name] = m
		status.MemberPeerMap[m.PeerURL] = m
		status.Members = append(status.Members, m)
		if node[0] == name {
			status.MemberSelf = m
		}
	}

	for _, n := range strings.Split(initialClusterClients, ",") {
		node := strings.Split(n, "=")
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
		return nil, fmt.Errorf("Member config not found for self (%s)", name)
	}

	status.PodSpec = func(initialClusterState string, runRestore bool) *v1.Pod {
		var memberID uint64
		if status.MemberSelf.MemberID != nil {
			memberID = *status.MemberSelf.MemberID
		}

		return podspec.Create(
			name, certFile, keyFile, trustedCAFile, peerCertFile, peerKeyFile, peerTrustedCAFile, initialAdvertisePeerURLs,
			listenPeerURLs, advertiseClientURLs, listenClientURLs, initialClusterToken, initialCluster,
			etcdImage, etcdPodName, etcdPodNamespace, etcdSnapshotFile, etcdPodManifestFile,
			initialClusterState, runRestore, memberID,
		)
	}
	return status, nil
}

func (v *Status) SyncStatus() error {
	v.MembersHealthy = []*Member{}
	v.Healthy = false
	v.BackupMemberID = nil

	var clients []string
	for _, m := range v.Members {
		clients = append(clients, m.ClientURL)
	}

	respCh, err := etcdutil.Status(clients, v.ClientTLSConfig)
	if err != nil {
		return err
	}

	clusterIDCounts := make(map[uint64]int)
	var count int

	for {
		resp := <- respCh
		count++
		m, ok := v.MemberClientMap[resp.Endpoint]
		if !ok || m == nil || resp.Err != nil {
			continue
		}

		// memberID := resp.Status.Header.MemberId
		clusterID := resp.Status.Header.ClusterId
		leaderID := resp.Status.Leader
		revision := resp.Status.Header.Revision

		if clusterID == 0 {
			continue
		}

		// pick consistent majority clusterID in case of split brain
		clusterIDCounts[clusterID]++
		if v.ClusterID == nil || clusterIDCounts[clusterID] > clusterIDCounts[*v.ClusterID] ||
			(clusterIDCounts[clusterID] == clusterIDCounts[*v.ClusterID] && clusterID < *v.ClusterID) {
			v.ClusterID = &clusterID
			v.LeaderID = &leaderID
			v.Revision = &revision
		}
		m.ClusterID = &clusterID
		m.LeaderID = &leaderID
		m.Revision = &revision

		if count >= len(clients) {
			close(respCh)
			break
		}
	}

	if v.ClusterID == nil {
		return nil
	}

	var clientsHealhty []string
	for _, m := range v.Members {
		if m.ClusterID == v.ClusterID {
			v.MembersHealthy = append(v.MembersHealthy, m)
			clientsHealhty = append(clientsHealhty, m.ClientURL)
		}
	}

	// run a list on healthy members to get non existent members to remove
	members, err := etcdutil.ListMembers(clientsHealhty, v.ClientTLSConfig)
	if err != nil {
		return err
	}

	// member Name field may not be populated right away
	// Match returned members by PeerURL field
	peerURLsReturned := make(map[string]struct{})
	for _, member := range members.Members {
		var m *Member
		var ok bool

		for _, peer := range member.PeerURLs {
			var memberID uint64
			if m, ok = v.MemberPeerMap[peer]; ok {
				memberID = member.ID
				if memberID != 0 {
					m.MemberIDFromCluster = &memberID
				}

				peerURLsReturned[peer] = struct{}{}
				break
			}
		}
	}

	// Compare returned members with list and remove inactive ones
	for peer, m := range v.MemberPeerMap {
		if _, ok := peerURLsReturned[peer]; !ok {
			m.MemberIDFromCluster = nil
		}
	}

	// check health status
	if err := etcdutil.HealthCheck(clientsHealhty, v.ClientTLSConfig); err != nil {
		return nil
	}

	v.Healthy = true

	// pick backup member
	for _, m := range v.MembersHealthy {
		if *m.Revision < *v.Revision {
			continue
		}

		if v.BackupMemberID == nil || *m.MemberID < *v.BackupMemberID {
			v.BackupMemberID = m.MemberID
		}
	}
	return nil
}

func (v *Status) ReplaceMember(m *Member) error {
	var clientsHealhty []string
	for _, m := range v.MembersHealthy {
		clientsHealhty = append(clientsHealhty, m.ClientURL)
	}

	err := etcdutil.RemoveMember(clientsHealhty, v.ClientTLSConfig, *m.MemberIDFromCluster)
	if err != nil {
		return err
	}
	resp, err := etcdutil.AddMember(clientsHealhty, v.ListenPeerURLs, v.ClientTLSConfig)
	if err != nil {
		return err
	}
	memberID := resp.Member.ID
	if memberID == 0 {
		return fmt.Errorf("add member returned member ID 0")
	}
	m.MemberID = &memberID
	m.MemberIDFromCluster = &memberID
	return nil
}

func (v *Status) WritePodManifest(runRestore bool) error {
	var pod *v1.Pod
	if runRestore {
		err := v.RestoreSnapshot()
		if err != nil {
			pod = v.PodSpec("new", false)
		} else {
			pod = v.PodSpec("existing", true)
		}
	} else {
		pod = v.PodSpec("existing", false)
	}

	manifest, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteFile(io.NopCloser(bytes.NewReader(manifest)), v.EtcdPodManifestFile)
}

func (v *Status) BackupSnapshot() error {
	if v.Healthy && v.BackupMemberID != nil && v.BackupMemberID == v.MemberSelf.MemberID {
		var clientsHealhty []string
		for _, m := range v.MembersHealthy {
			clientsHealhty = append(clientsHealhty, m.ClientURL)
		}

		sess := session.Must(session.NewSession(&aws.Config{}))
		err := etcdutil.BackupSnapshot(clientsHealhty, v.S3BackupResource, s3util.NewWriter(s3.New(sess)), v.ClientTLSConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Status) RestoreSnapshot() error {
	sess := session.Must(session.NewSession(&aws.Config{}))
	err := etcdutil.RestoreSnapshot(v.EtcdSnapshotFile, v.S3BackupResource, s3util.NewReader(s3.New(sess)))
	if err != nil {
		return err
	}
	return nil
}
