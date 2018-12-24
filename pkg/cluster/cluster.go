package cluster

import (
	"flag"
	"strconv"
	"strings"
	"time"
)

type Cluster struct {
	// Args for etcd
	Name                     string
	CertFile                 string
	KeyFile                  string
	TrustedCAFile            string
	PeerCertFile             string
	PeerKeyFile              string
	PeerTrustedCAFile        string
	InitialAdvertisePeerURLs string
	ListenPeerURLs           string
	AdvertiseClientURLs      string
	ListenClientURLs         string
	InitialClusterToken      string
	InitialCluster           string

	// Update this in pod spec annotation to restart etcd pod
	Instance string
	// Mount this to run etcdctl snapshot restore
	BackupMountDir string
	// This path should be under BackupMountDir
	BackupFile string
	// Mount this in etcd container - cert files should all be under this path
	EtcdTLSMountDir string
	// List of etcd client URLs for service to hit
	EtcdServers string
	// etcd image
	Image string
	// kubelet static pod path
	PodSpecFile  string
	S3BackupPath string

	// etcd node ID
	ID uint64

	// Main loop interval
	RunInterval time.Duration
	BackupInterval  time.Duration
	MemberWait time.Duration
	PodSpecWait time.Duration
}

func NewCluster() *Cluster {
	cluster := &Cluster{}
	// Args for etcd
	flag.StringVar(&cluster.Name, "name", "", "Human-readable name for this member.")
	flag.StringVar(&cluster.CertFile, "cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&cluster.KeyFile, "key-file", "", "Path to the client server TLS key file.")
	flag.StringVar(&cluster.TrustedCAFile, "trusted-ca-file", "", "Path to the client server TLS trusted CA cert file.")
	flag.StringVar(&cluster.PeerCertFile, "peer-cert-file", "", "Path to the peer server TLS cert file.")
	flag.StringVar(&cluster.PeerKeyFile, "peer-key-file", "", "Path to the peer server TLS key file.")
	flag.StringVar(&cluster.PeerTrustedCAFile, "peer-trusted-ca-file", "", "Path to the peer server TLS trusted CA file.")
	flag.StringVar(&cluster.InitialAdvertisePeerURLs, "initial-advertise-peer-urls", "", "List of this member's peer URLs to advertise to the rest of the cluster.")
	flag.StringVar(&cluster.ListenPeerURLs, "listen-peer-urls", "", "List of URLs to listen on for peer traffic.")
	flag.StringVar(&cluster.AdvertiseClientURLs, "advertise-client-urls", "", "List of this member's client URLs to advertise to the public.")
	flag.StringVar(&cluster.ListenClientURLs, "listen-client-urls", "", "List of URLs to listen on for client traffic.")
	flag.StringVar(&cluster.InitialClusterToken, "initial-cluster-token", "", "Initial cluster token for the etcd cluster during bootstrap.")
	flag.StringVar(&cluster.InitialCluster, "initial-cluster", "", "Initial cluster configuration for bootstrapping.")
	//
	flag.StringVar(&cluster.BackupMountDir, "backup-dir", "/var/lib/etcd-restore", "Base path of snapshot restore file.")
	flag.StringVar(&cluster.BackupFile, "backup-file", "/var/lib/etcd-restore/etcd.db", "Snapshot file restore path.")
	flag.StringVar(&cluster.EtcdTLSMountDir, "tls-dir", "/etc/ssl/cert", "Base path of TLS cert files.")
	flag.StringVar(&cluster.EtcdServers, "etcd-servers", "", "List of etcd client URLs.")
	flag.StringVar(&cluster.Image, "image", "quay.io/coreos/etcd:v3.3", "Etcd container image.")
	flag.StringVar(&cluster.PodSpecFile, "pod-spec-file", "", "Pod spec file path (intended to be in kubelet manifests path).")
	flag.StringVar(&cluster.S3BackupPath, "s3-backup-path", "", "S3 key name for backup.")
	
	flag.DurationVar(&cluster.RunInterval, "run-interval", 10, "Member check interval.")
	flag.DurationVar(&cluster.BackupInterval, "backup-interval", 300, "Backup trigger interval.")
	flag.DurationVar(&cluster.MemberWait, "member-wait", 300, "Wait after member configuration.")
	flag.DurationVar(&cluster.PodSpecWait, "podspec-wait", 300, "Wait after writing pod spec.")
	flag.Parse()
	return cluster
}

func MemberURLsFromConfig(c *Cluster) []string {
	var memberList []string
	for _, m := range strings.Split(c.InitialCluster, ",") {
		memberList = append(memberList, strings.Split(m, "=")[0])
	}
	return memberList
}

func ClientURLsFromConfig(c *Cluster) []string {
	return strings.Split(c.EtcdServers, ",")
}

func LocalPeerURLsFromConfig(c *Cluster) []string {
	return strings.Split(c.ListenPeerURLs, ",")
}

func LocalClientURLsFromConfig(c *Cluster) []string {
	return strings.Split(c.ListenClientURLs, ",")
}

// Change annotation in pod to force update
func (c *Cluster) UpdateInstance() {
	c.Instance = strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}