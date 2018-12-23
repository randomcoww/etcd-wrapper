package cluster

import (
	"strings"
	"time"
)

type Cluster struct {
	Name     string
	Instance string

	// Mount this to run etcdctl snapshot restore
	BackupMountDir string
	// This path should be under BackupMountDir
	BackupFile string

	// Mount this in etcd container - cert files should all be under this path
	EtcdTLSMountDir string
	// These paths should be under EtcdTLSMountDir
	CertFile          string
	KeyFile           string
	TrustedCAFile     string
	PeerCertFile      string
	PeerKeyFile       string
	PeerTrustedCAFile string

	// Listen
	InitialAdvertisePeerURLs string
	ListenPeerURLs           string
	AdvertiseClientURLs      string
	ListenClientURLs         string

	InitialClusterToken string
	InitialCluster      string

	EtcdServers string
	// etcd image
	Image string
	// kubelet static pod path
	PodSpecFile  string
	S3BackupPath string

	RunBackup chan struct{}
	ClusterError chan struct{}

	RunInterval    time.Duration
	BackupInterval time.Duration
	EtcdTimeout    time.Duration
}

func ClientURLsFromConfig(c *Cluster) []string {
	return strings.Split(c.EtcdServers, ",")
}

// func PeerURLsFromConfig(c *Cluster) []string {
// 	var peerURLs []string
// 	for _, m := range strings.Split(c.InitialCluster, ",") {
// 		peerURLs = append(peerURLs, strings.Split(m, "=")[1])
// 	}
// 	return peerURLs
// }

func ListenPeerURLsFromConfig(c *Cluster) []string {
	return strings.Split(c.ListenPeerURLs, ",")
}
