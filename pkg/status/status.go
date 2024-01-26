package status

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	S3BackupResource            string        `yaml:"-"`
	EtcdSnapshotFile            string        `yaml:"-"`
	EtcdPodManifestFile         string        `yaml:"-"`
	ListenPeerURLs              []string      `yaml:"-"`
	HealthCheckInterval         time.Duration `yaml:"-"`
	BackupInterval              time.Duration `yaml:"-"`
	HealthCheckFailCountAllowed int           `yaml:"-"`
	ReadinessFailCountAllowed   int           `yaml:"-"`
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
	var clientCertFile, clientKeyFile, initialClusterClients, s3BackupResource string
	var healthCheckInterval, backupInterval time.Duration
	var healthCheckFailCountAllowed, readinessFailCountAllowed int
	flag.StringVar(&clientCertFile, "client-cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&clientKeyFile, "client-key-file", "", "Path to the client server TLS key file.")
	flag.StringVar(&initialClusterClients, "initial-cluster-clients", "", "List of etcd nodes and client URLs in same format as intial-cluster.")
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

	status.S3BackupResource = s3BackupResource
	status.EtcdSnapshotFile = etcdSnapshotFile
	status.EtcdPodManifestFile = etcdPodManifestFile

	status.MemberMap = make(map[string]*Member)
	status.MemberPeerMap = make(map[string]*Member)
	status.MemberClientMap = make(map[string]*Member)

	status.ListenPeerURLs = strings.Split(listenPeerURLs, ",")
	status.HealthCheckInterval = healthCheckInterval
	status.BackupInterval = backupInterval
	status.HealthCheckFailCountAllowed = healthCheckFailCountAllowed
	status.ReadinessFailCountAllowed = readinessFailCountAllowed

	for _, n := range strings.Split(initialCluster, ",") {
		node := strings.Split(n, "=")
		m := &Member{
			Name:    node[0],
			PeerURL: node[1],
		}
		status.MemberMap[node[0]] = m
		status.MemberPeerMap[node[1]] = m
		status.Members = append(status.Members, m)
		if node[0] == name {
			status.MemberSelf = m
		}
	}

	for _, n := range strings.Split(initialClusterClients, ",") {
		node := strings.Split(n, "=")
		m, ok := status.MemberMap[node[0]]
		if !ok {
			return nil, fmt.Errorf("Mismatch in initial-cluster and initial-cluster-clients members")
		}
		m.ClientURL = node[1]
		status.MemberClientMap[node[1]] = m
	}

	if len(status.MemberClientMap) != len(status.MemberMap) {
		return nil, fmt.Errorf("Mismatch in initial-cluster and initial-cluster-clients members")
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

func (v *Status) SyncStatus() error {
	v.clearState()

	var clients []string
	for _, m := range v.Members {
		m.clearState()
		clients = append(clients, m.ClientURL)
	}

	// check health status
	err := etcdutil.HealthCheck(clients, v.ClientTLSConfig)
	if err != nil {
		return nil
	}
	respCh, err := etcdutil.Status(clients, v.ClientTLSConfig)
	if err != nil {
		return nil
	}

	clusterMap := make(map[uint64]int)
	var count int

	for {
		if count >= len(clients) {
			close(respCh)
			break
		}

		resp := <-respCh
		count++

		m, ok := v.MemberClientMap[resp.Endpoint]
		if !ok || m == nil {
			continue
		}
		if resp.Err != nil {
			continue
		}

		clusterID := resp.Status.Header.ClusterId
		if clusterID == 0 {
			continue
		}

		memberID := resp.Status.Header.MemberId
		revision := resp.Status.Header.Revision
		leaderID := resp.Status.Leader

		// pick consistent majority clusterID in case of split brain
		clusterMap[clusterID]++
		if v.ClusterID == nil || clusterMap[clusterID] > clusterMap[*v.ClusterID] ||
			(clusterMap[clusterID] == clusterMap[*v.ClusterID] && clusterID < *v.ClusterID) {
			v.ClusterID = &clusterID
			v.LeaderID = &leaderID
			v.Revision = &revision
		}
		m.ClusterID = &clusterID
		m.LeaderID = &leaderID
		m.Revision = &revision
		m.MemberID = &memberID
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
	members, err := etcdutil.ListMembers(clientsHealhty, v.ClientTLSConfig)
	if err != nil {
		return err
	}
	// member Name field may not be populated right away
	// Match returned members by PeerURL field
	for _, member := range members.Members {
		if member.ID != 0 {
			memberID := member.ID
			for _, peer := range member.PeerURLs {
				if m, ok := v.MemberPeerMap[peer]; ok {
					m.MemberIDFromCluster = &memberID
					break
				}
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
	resp, err := etcdutil.AddMember(clientsHealhty, v.ListenPeerURLs, v.ClientTLSConfig)
	if err != nil {
		return err
	}
	memberID := resp.Member.ID
	if memberID == 0 {
		return fmt.Errorf("Add member returned ID 0")
	}
	m.MemberID = &memberID
	m.MemberIDFromCluster = &memberID
	return nil
}

func (v *Status) WritePodManifest(runRestore bool) error {
	var pod *v1.Pod
	manifestVersion := fmt.Sprintf("%v", time.Now().Unix())
	if runRestore {
		ok, err := v.RestoreSnapshot()
		if err != nil {
			log.Printf("Read S3 snapshot resource failed: %v", err)
			return err
		}
		if ok {
			log.Printf("Snapshot pull succeeded. Restoring cluster")
			pod = v.PodSpec("existing", true, manifestVersion)
		} else {
			log.Printf("Snapshot not found. Starting new cluster")
			pod = v.PodSpec("new", false, manifestVersion)
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

func (v *Status) BackupSnapshot() error {
	if v.Healthy && v.BackupMemberID != nil && v.BackupMemberID == v.MemberSelf.MemberID {
		var clientsHealhty []string
		for _, m := range v.MembersHealthy {
			clientsHealhty = append(clientsHealhty, m.ClientURL)
		}
		cfg, err := config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return err
		}
		err = etcdutil.BackupSnapshot(clientsHealhty, v.S3BackupResource, s3util.New(s3.NewFromConfig(cfg)), v.ClientTLSConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Status) RestoreSnapshot() (bool, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return false, err
	}
	ok, err := etcdutil.RestoreSnapshot(v.EtcdSnapshotFile, v.S3BackupResource, s3util.New(s3.NewFromConfig(cfg)))
	if err != nil {
		return false, err
	}
	return ok, nil
}
