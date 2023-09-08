package config

import (
	"crypto/tls"
	"flag"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"io/ioutil"
	"strings"
	"time"
)

type Config struct {
	// -- Params to pass to etcd
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

	// -- Params for etcd and data recovery pod creation
	// Pod name for etcd
	EtcdPodName      string
	EtcdPodNamespace string
	// Mount this to run etcdctl snapshot restore
	BackupFile string
	// etcd image
	EtcdImage string
	// kubelet static pod path
	PodSpecFile string

	// -- Params for etcd wrapper
	// Client cert for getting status of all etcd nodes (not just local)
	ClientCertFile string
	ClientKeyFile  string
	// List of etcd client URLs for service to hit
	ClusterClientURLs string
	// S3 bucket and key for etcd backups
	S3BackupPath string
	// Main loop interval
	BackupInterval           time.Duration
	HealthCheckInterval      time.Duration
	HealthCheckFailuresAllow int
	PodUpdateWait            time.Duration

	// Parsed static values
	TLSConfig       *tls.Config
	MemberNames     []string
	PeerURLs        []string
	ClientURLs      []string
	LocalPeerURLs   []string
	LocalClientURLs []string
}

func NewConfig() (*Config, error) {
	config := &Config{}
	// Etcd config to pass
	flag.StringVar(&config.Name, "name", "", "Human-readable name for this member.")
	flag.StringVar(&config.CertFile, "host-cert-file", "", "Host path to the client server TLS cert file.")
	flag.StringVar(&config.KeyFile, "host-key-file", "", "Host path to the client server TLS key file.")
	flag.StringVar(&config.TrustedCAFile, "host-trusted-ca-file", "", "Host path to the client server TLS trusted CA cert file.")
	flag.StringVar(&config.PeerCertFile, "host-peer-cert-file", "", "Host path to the peer server TLS cert file.")
	flag.StringVar(&config.PeerKeyFile, "host-peer-key-file", "", "Host path to the peer server TLS key file.")
	flag.StringVar(&config.PeerTrustedCAFile, "host-peer-trusted-ca-file", "", "Host path to the peer server TLS trusted CA file.")
	flag.StringVar(&config.InitialAdvertisePeerURLs, "initial-advertise-peer-urls", "", "List of this member's peer URLs to advertise to the rest of the cluster.")
	flag.StringVar(&config.ListenPeerURLs, "listen-peer-urls", "", "List of URLs to listen on for peer traffic.")
	flag.StringVar(&config.AdvertiseClientURLs, "advertise-client-urls", "", "List of this member's client URLs to advertise to the public.")
	flag.StringVar(&config.ListenClientURLs, "listen-client-urls", "", "List of URLs to listen on for client traffic.")
	flag.StringVar(&config.InitialClusterToken, "initial-cluster-token", "", "Initial cluster token for the etcd cluster during bootstrap.")
	flag.StringVar(&config.InitialCluster, "initial-cluster", "", "Initial cluster configuration for bootstrapping.")
	// Etcd pod creation config
	flag.StringVar(&config.EtcdPodName, "etcd-pod-name", "etcd", "Name of etcd pod.")
	flag.StringVar(&config.EtcdPodNamespace, "etcd-pod-namespace", "kube-system", "Namespace to launch etcd pod.")
	flag.StringVar(&config.BackupFile, "host-backup-file", "/var/lib/etcd-restore/etcd.db", "Host path to restore snapshot file.")
	flag.StringVar(&config.PodSpecFile, "host-etcd-manifest-file", "", "Host path to write etcd pod manifest file. This should be where kubelet reads static pod manifests.")
	flag.StringVar(&config.EtcdImage, "etcd-image", "", "Etcd container image.")
	// Etcd wrapper config
	flag.StringVar(&config.ClientCertFile, "client-cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&config.ClientKeyFile, "client-key-file", "", "Path to the client server TLS key file.")
	flag.StringVar(&config.ClusterClientURLs, "cluster-client-urls", "", "List of etcd client URLs.")
	flag.StringVar(&config.S3BackupPath, "s3-backup-path", "", "S3 key name for backup.")
	flag.DurationVar(&config.BackupInterval, "backup-interval", 30*time.Minute, "Backup trigger interval.")
	flag.DurationVar(&config.HealthCheckInterval, "healthcheck-interval", 5*time.Second, "Healthcheck interval.")
	flag.IntVar(&config.HealthCheckFailuresAllow, "healthcheck-failures-allow", 3, "Number of healthcheck failures to allow before updating etcd pod.")
	flag.DurationVar(&config.PodUpdateWait, "pod-update-wait", 30*time.Second, "Time to wait after pod manifest update to resume health checks.")
	flag.Parse()

	if err := config.addParsedTLS(); err != nil {
		return nil, err
	}
	config.addParsedConfig()
	return config, nil
}

func (c *Config) addParsedTLS() error {
	cert, _ := ioutil.ReadFile(c.ClientCertFile)
	key, _ := ioutil.ReadFile(c.ClientKeyFile)
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
	// List of all peer URLs
	c.PeerURLs = []string{}
	// List of client URLs
	c.ClientURLs = strings.Split(c.ClusterClientURLs, ",")
	// List of peer URLs of local node
	c.LocalPeerURLs = strings.Split(c.ListenPeerURLs, ",")
	// List of client URLs of local node
	c.LocalClientURLs = strings.Split(c.ListenClientURLs, ",")

	for _, m := range strings.Split(c.InitialCluster, ",") {
		node := strings.Split(m, "=")
		c.MemberNames = append(c.MemberNames, node[0])
		c.PeerURLs = append(c.PeerURLs, node[1])
	}
}

// Compare URL lists
// https://stackoverflow.com/questions/15311969/checking-the-equality-of-two-slices
func IsEqual(a, b []string) bool {

	// If one is nil, the other must also be nil.
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
