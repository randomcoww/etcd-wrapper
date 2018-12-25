package config

import (
	"crypto/tls"
	"flag"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
)

type Config struct {
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

	// Main loop interval
	BackupInterval      time.Duration
	HealthCheckInterval time.Duration
	LocalErrThreshold   int
	ClusterErrThreshold int

	// Parsed static values
	TLSConfig       *tls.Config
	MemberNames     []string
	PeerURLs        []string
	ClientURLs      []string
	LocalPeerURLs   []string
	LocalClientURLs []string

	// Healthcheck reporting
	NotifyMissingNew      chan struct{}
	NotifyMissingExisting chan struct{}
	NotifyRemoteRemove    chan uint64
	NotifyLocalRemove     chan uint64
}

func NewConfig() (*Config, error) {
	config := &Config{
		NotifyMissingNew:      make(chan struct{}, 1),
		NotifyMissingExisting: make(chan struct{}, 1),
		NotifyRemoteRemove:    make(chan uint64),
		NotifyLocalRemove:     make(chan uint64, 1),
	}
	// Args for etcd
	flag.StringVar(&config.Name, "name", "", "Human-readable name for this member.")
	flag.StringVar(&config.CertFile, "cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&config.KeyFile, "key-file", "", "Path to the client server TLS key file.")
	flag.StringVar(&config.TrustedCAFile, "trusted-ca-file", "", "Path to the client server TLS trusted CA cert file.")
	flag.StringVar(&config.PeerCertFile, "peer-cert-file", "", "Path to the peer server TLS cert file.")
	flag.StringVar(&config.PeerKeyFile, "peer-key-file", "", "Path to the peer server TLS key file.")
	flag.StringVar(&config.PeerTrustedCAFile, "peer-trusted-ca-file", "", "Path to the peer server TLS trusted CA file.")
	flag.StringVar(&config.InitialAdvertisePeerURLs, "initial-advertise-peer-urls", "", "List of this member's peer URLs to advertise to the rest of the cluster.")
	flag.StringVar(&config.ListenPeerURLs, "listen-peer-urls", "", "List of URLs to listen on for peer traffic.")
	flag.StringVar(&config.AdvertiseClientURLs, "advertise-client-urls", "", "List of this member's client URLs to advertise to the public.")
	flag.StringVar(&config.ListenClientURLs, "listen-client-urls", "", "List of URLs to listen on for client traffic.")
	flag.StringVar(&config.InitialClusterToken, "initial-cluster-token", "", "Initial cluster token for the etcd cluster during bootstrap.")
	flag.StringVar(&config.InitialCluster, "initial-cluster", "", "Initial cluster configuration for bootstrapping.")
	// Client
	flag.StringVar(&config.BackupMountDir, "backup-dir", "/var/lib/etcd-restore", "Base path of snapshot restore file.")
	flag.StringVar(&config.BackupFile, "backup-file", "/var/lib/etcd-restore/etcd.db", "Snapshot file restore path.")
	flag.StringVar(&config.EtcdTLSMountDir, "tls-dir", "/etc/ssl/cert", "Base path of TLS cert files.")
	flag.StringVar(&config.EtcdServers, "etcd-servers", "", "List of etcd client URLs.")
	flag.StringVar(&config.Image, "image", "quay.io/coreos/etcd:v3.3", "Etcd container image.")
	flag.StringVar(&config.PodSpecFile, "pod-spec-file", "", "Pod spec file path (intended to be in kubelet manifests path).")
	flag.StringVar(&config.S3BackupPath, "s3-backup-path", "", "S3 key name for backup.")

	flag.DurationVar(&config.BackupInterval, "backup-interval", 300, "Backup trigger interval.")
	flag.DurationVar(&config.HealthCheckInterval, "healthcheck-interval", 10, "Healthcheck interval.")
	flag.IntVar(&config.LocalErrThreshold, "local-err-thresh", 3, "Error count to trigger local member missing error.")
	flag.IntVar(&config.ClusterErrThreshold, "cluster-err-thresh", 3, "Error count to trigger cluster error.")
	// flag.IntVar(&config.MemberErrThreshold, "member-err-thresh", 2, "Error count to trigger member add or remove error.")
	flag.Parse()

	if err := config.addParsedTLS(); err != nil {
		return nil, err
	}
	config.addParsedConfig()

	return config, nil
}

// Change annotation in pod to force update
func (c *Config) UpdateInstance() {
	c.Instance = strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}

func (c *Config) addParsedTLS() error {
	cert, _ := ioutil.ReadFile(c.CertFile)
	key, _ := ioutil.ReadFile(c.KeyFile)
	ca, _ := ioutil.ReadFile(c.TrustedCAFile)

	tc, err := etcdutil.NewTLSConfig(cert, key, ca)
	if err != nil {
		return err
	}

	c.TLSConfig = tc
	return nil
}

func (c *Config) addParsedConfig() {
	// List of client names
	c.MemberNames = []string{}
	// List of peer URLs
	c.PeerURLs = []string{}
	for _, m := range strings.Split(c.InitialCluster, ",") {
		node := strings.Split(m, "=")
		c.MemberNames = append(c.MemberNames, node[0])
		c.PeerURLs = append(c.PeerURLs, node[1])
	}

	// List of client URLs
	c.ClientURLs = strings.Split(c.EtcdServers, ",")
	// List of peer URLs of local node
	c.LocalPeerURLs = strings.Split(c.ListenPeerURLs, ",")
	// List of client URLs of local node
	c.LocalClientURLs = strings.Split(c.ListenClientURLs, ",")
}
