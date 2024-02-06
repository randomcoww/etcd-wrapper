package status

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/podspec"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/util/s3util"
	"go.etcd.io/etcd/pkg/transport"
	"gopkg.in/yaml.v3"
	"io"
	"k8s.io/api/core/v1"
	"log"
	"strings"
	"time"
)

type Member struct {
	Name                string  `yaml:"-"`
	Healthy             bool    `yaml:"healthy"`
	MemberID            *uint64 `yaml:"memberID,omitempty"`
	MemberIDFromCluster *uint64 `yaml:"memberIDFromCluster,omitempty"`
	Revision            *int64  `yaml:"revision,omitempty"`
	ClusterID           *uint64 `yaml:"clusterID,omitempty"`
	LeaderID            *uint64 `yaml:"leaderID,omitempty"`
	Self                bool    `yaml:"self,omitempty"`
	//
	PeerURL   string `yaml:"-"`
	ClientURL string `yaml:"-"`
}

type Status struct {
	Healthy        bool    `yaml:"healthy"`
	ClusterID      *uint64 `yaml:"clusterID,omitempty"`
	LeaderID       *uint64 `yaml:"leaderID,omitempty"`
	BackupMemberID *uint64 `yaml:"backupMemberID,omitempty"`
	Revision       *int64  `yaml:"revision,omitempty"`
	//
	MemberMap       map[string]*Member                 `yaml:"members"`
	MemberPeerMap   map[string]*Member                 `yaml:"-"`
	MemberClientMap map[string]*Member                 `yaml:"-"`
	MemberSelf      *Member                            `yaml:"-"`
	Members         []*Member                          `yaml:"-"`
	MembersHealthy  []*Member                          `yaml:"-"`
	ClientTLSConfig *tls.Config                        `yaml:"-"`
	PodSpec         func(string, bool, string) *v1.Pod `yaml:"-"`
	//
	s3Client                    *s3util.Client `yaml:"-"`
	S3BackupBucket              string         `yaml:"-"`
	S3BackupKey                 string         `yaml:"-"`
	EtcdSnapshotFile            string         `yaml:"-"`
	EtcdPodManifestFile         string         `yaml:"-"`
	ListenPeerURLs              []string       `yaml:"-"`
	HealthCheckInterval         time.Duration  `yaml:"-"`
	BackupInterval              time.Duration  `yaml:"-"`
	HealthCheckFailCountAllowed int            `yaml:"-"`
	ReadinessFailCountAllowed   int            `yaml:"-"`
}

func New() (*Status, error) {
	status := &Status{}
	var err error

	// etcd args
	var name, certFile, keyFile, trustedCAFile, peerCertFile, peerKeyFile, peerTrustedCAFile, initialAdvertisePeerURLs, listenPeerURLs, advertiseClientURLs, listenClientURLs, initialClusterToken, initialCluster, autoCompationRetention string
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
	flag.StringVar(&autoCompationRetention, "auto-compaction-retention", "0", "Auto compaction retention length. 0 means disable auto compaction.")

	// pod manifest args
	var etcdImage, etcdPodName, etcdPodNamespace, etcdSnapshotFile, etcdPodManifestFile string
	flag.StringVar(&etcdImage, "etcd-image", "", "Etcd container image.")
	flag.StringVar(&etcdPodName, "etcd-pod-name", "etcd", "Name of etcd pod.")
	flag.StringVar(&etcdPodNamespace, "etcd-pod-namespace", "kube-system", "Namespace to launch etcd pod.")
	flag.StringVar(&etcdSnapshotFile, "etcd-snaphot-file", "/var/lib/etcd/etcd.db", "Host path to restore snapshot file.")
	flag.StringVar(&etcdPodManifestFile, "etcd-pod-manifest-file", "", "Host path to write etcd pod manifest file. This should be where kubelet reads static pod manifests.")

	// etcd wrapper args
	var clientCertFile, clientKeyFile, initialClusterClients, s3BackupEndpoint, s3BackupResource string
	var healthCheckInterval, backupInterval time.Duration
	var healthCheckFailCountAllowed, readinessFailCountAllowed int
	flag.StringVar(&clientCertFile, "client-cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&clientKeyFile, "client-key-file", "", "Path to the client server TLS key file.")
	flag.StringVar(&initialClusterClients, "initial-cluster-clients", "", "List of etcd nodes and client URLs in same format as intial-cluster.")
	flag.StringVar(&s3BackupEndpoint, "s3-backup-endpoint", "s3.amazonaws.com", "S3 endpoint for backup.")
	flag.StringVar(&s3BackupResource, "s3-backup-resource", "", "S3 resource name for backup.")
	flag.DurationVar(&healthCheckInterval, "healthcheck-interval", 6*time.Second, "Healthcheck interval.")
	flag.DurationVar(&backupInterval, "backup-interval", 15*time.Minute, "Backup trigger interval.")
	flag.IntVar(&healthCheckFailCountAllowed, "healthcheck-fail-count-allowed", 16, "Number of healthcheck failures to allow before restarting etcd pod.")
	flag.IntVar(&readinessFailCountAllowed, "readiness-fail-count-allowed", 64, "Number of readiness check failures to allow before restarting etcd pod.")
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

	status.EtcdSnapshotFile = etcdSnapshotFile
	status.EtcdPodManifestFile = etcdPodManifestFile

	status.ListenPeerURLs = strings.Split(listenPeerURLs, ",")
	status.HealthCheckInterval = healthCheckInterval
	status.BackupInterval = backupInterval
	status.HealthCheckFailCountAllowed = healthCheckFailCountAllowed
	status.ReadinessFailCountAllowed = readinessFailCountAllowed

	status.s3Client, err = s3util.New(s3BackupEndpoint)
	if err != nil {
		return nil, err
	}
	status.S3BackupBucket, status.S3BackupKey, err = s3util.ParseBucketAndKey(s3BackupResource)
	if err != nil {
		return nil, err
	}
	if err = status.populateMembersFromInitialCluster(name, initialCluster, initialClusterClients); err != nil {
		return nil, err
	}

	status.PodSpec = func(initialClusterState string, runRestore bool, versionAnnotation string) *v1.Pod {
		var memberID uint64
		if status.MemberSelf.MemberID != nil {
			memberID = *status.MemberSelf.MemberID
		}

		return podspec.Create(
			name, certFile, keyFile, trustedCAFile, peerCertFile, peerKeyFile, peerTrustedCAFile, initialAdvertisePeerURLs,
			listenPeerURLs, advertiseClientURLs, listenClientURLs, initialClusterToken, initialCluster,
			etcdImage, etcdPodName, etcdPodNamespace, etcdSnapshotFile, autoCompationRetention,
			initialClusterState, runRestore, memberID, versionAnnotation,
		)
	}
	return status, nil
}

func (v *Status) populateMembersFromInitialCluster(name, initialCluster, initialClusterClients string) error {
	v.MemberMap = make(map[string]*Member)
	v.MemberPeerMap = make(map[string]*Member)
	v.MemberClientMap = make(map[string]*Member)

	for _, n := range strings.Split(initialCluster, ",") {
		node := strings.Split(n, "=")
		m := &Member{
			Name:    node[0],
			PeerURL: node[1],
		}
		v.MemberMap[node[0]] = m
		v.MemberPeerMap[node[1]] = m
		v.Members = append(v.Members, m)
		if node[0] == name {
			m.Self = true
			v.MemberSelf = m
		}
	}

	for _, n := range strings.Split(initialClusterClients, ",") {
		node := strings.Split(n, "=")
		m, ok := v.MemberMap[node[0]]
		if !ok {
			return fmt.Errorf("Mismatch in initial-cluster and initial-cluster-clients members")
		}
		m.ClientURL = node[1]
		v.MemberClientMap[node[1]] = m
	}

	if len(v.MemberClientMap) != len(v.MemberMap) {
		return fmt.Errorf("Mismatch in initial-cluster and initial-cluster-clients members")
	}
	return nil
}

func (v *Status) clearState() {
	v.MembersHealthy = []*Member{}
	v.Healthy = false
	v.ClusterID = nil
	v.LeaderID = nil
	v.Revision = nil
	v.BackupMemberID = nil
}

func (v *Status) ToYaml() (b []byte, err error) {
	b, err = yaml.Marshal(v)
	return
}

func (v *Member) clearState() {
	v.Healthy = false
	v.MemberID = nil
	v.MemberIDFromCluster = nil
	v.ClusterID = nil
	v.LeaderID = nil
	v.Revision = nil
}

func (v *Status) SyncStatus(etcd etcdutil.StatusCheck) error {
	v.clearState()

	var clients []string
	for _, m := range v.Members {
		m.clearState()
		clients = append(clients, m.ClientURL)
	}

	// check health status
	err := etcd.HealthCheck(clients, v.ClientTLSConfig)
	if err != nil {
		return nil
	}

	clusterIDCount := make(map[uint64]int)
	err = etcd.Status(clients, func(status *etcdutil.StatusResp) {
		m, ok := v.MemberClientMap[status.Endpoint]
		if !ok || m == nil {
			return
		}
		m.ClusterID = status.ClusterID
		m.LeaderID = status.LeaderID
		m.Revision = status.Revision
		m.MemberID = status.MemberID

		// set cluster wide IDs by majority found among members
		clusterIDCount[*status.ClusterID]++
		if v.ClusterID == nil ||
			clusterIDCount[*status.ClusterID] > clusterIDCount[*v.ClusterID] ||
			(clusterIDCount[*status.ClusterID] == clusterIDCount[*v.ClusterID] && *status.ClusterID < *v.ClusterID) {
			v.ClusterID = status.ClusterID
			v.LeaderID = status.LeaderID
			v.Revision = status.Revision
		}
	}, v.ClientTLSConfig)
	if err != nil {
		return nil
	}

	if v.ClusterID == nil {
		return nil
	}

	var clientsHealhty []string
	for _, m := range v.Members {
		if m.ClusterID == nil {
			continue
		}
		if *m.ClusterID == *v.ClusterID {
			v.MembersHealthy = append(v.MembersHealthy, m)
			clientsHealhty = append(clientsHealhty, m.ClientURL)
		}
	}

	// run a list on healthy members to get non existent members to remove
	members, err := etcd.ListMembers(clientsHealhty, v.ClientTLSConfig)
	if err != nil {
		return err
	}
	// member Name field may not be populated right away
	// Match returned members by PeerURL field
	for _, member := range members {
		for _, peer := range member.PeerURLs {
			if m, ok := v.MemberPeerMap[peer]; ok {
				m.MemberIDFromCluster = member.ID
				break
			}
		}
	}

	v.Healthy = true

	for _, m := range v.MembersHealthy {
		m.Healthy = m.MemberID != nil && m.MemberIDFromCluster != nil && *m.MemberID == *m.MemberIDFromCluster

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

	if m.MemberIDFromCluster != nil {
		err := etcdutil.RemoveMember(clientsHealhty, v.ClientTLSConfig, *m.MemberIDFromCluster)
		if err != nil {
			return err
		}
		log.Printf("Removed member %v", *m.MemberIDFromCluster)
	}
	member, err := etcdutil.AddMember(clientsHealhty, v.ListenPeerURLs, v.ClientTLSConfig)
	if err != nil {
		return err
	}
	m.MemberID = member.ID
	m.MemberIDFromCluster = member.ID
	return nil
}

func (v *Status) WritePodManifest(runRestore bool) error {
	var pod *v1.Pod
	manifestVersion := fmt.Sprintf("%v", time.Now().Unix())
	if runRestore {
		ok, err := v.RestoreSnapshot()
		if err != nil {
			return fmt.Errorf("Error getting snapshot: %v", err)
		}
		if !ok {
			log.Printf("Snapshot not found. Starting new cluster")
			pod = v.PodSpec("new", false, manifestVersion)
		} else {
			log.Printf("Successfully got snapshot. Restoring cluster")
			pod = v.PodSpec("existing", true, manifestVersion)
		}
	} else {
		pod = v.PodSpec("existing", false, manifestVersion)
	}

	manifest, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteFile(io.NopCloser(bytes.NewReader(manifest)), v.EtcdPodManifestFile)
}

func (v *Status) DeletePodManifest() error {
	return util.DeleteFile(v.EtcdPodManifestFile)
}

func (v *Status) Defragment() error {
	if v.Healthy {
		err := etcdutil.Defragment(v.MemberSelf.ClientURL, v.ClientTLSConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Status) BackupSnapshot() error {
	if v.Healthy && v.BackupMemberID != nil && v.BackupMemberID == v.MemberSelf.MemberID {
		var clientsHealhty []string
		for _, m := range v.MembersHealthy {
			clientsHealhty = append(clientsHealhty, m.ClientURL)
		}

		return etcdutil.CreateSnapshot(clientsHealhty, v.ClientTLSConfig, func(ctx context.Context, r io.Reader) error {
			return v.s3Client.Upload(ctx, v.S3BackupBucket, v.S3BackupKey, r)
		})
	}
	return nil
}

func (v *Status) RestoreSnapshot() (bool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return v.s3Client.Download(ctx, v.S3BackupBucket, v.S3BackupKey, func(ctx context.Context, r io.Reader) error {
		return util.WriteFile(r, v.EtcdSnapshotFile)
	})
}
