package arg

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/s3util"
	"go.etcd.io/etcd/pkg/transport"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"regexp"
	"time"
)

type Args struct {
	Name                     string
	CertFile                 string
	KeyFile                  string
	TrustedCAFile            string
	PeerCertFile             string
	PeerKeyFile              string
	PeerTrustedCAFile        string
	InitialClusterToken      string
	InitialClusterState      string
	InitialAdvertisePeerURLs []string
	ListenPeerURLs           []string
	AdvertiseClientURLs      []string
	ListenClientURLs         []string
	InitialCluster           []*Node

	// etcd wrapper args
	EtcdPod                   *v1.Pod
	EtcdSnapshotFile          string
	S3BackupBucket            string
	S3BackupKey               string
	HealthCheckInterval       time.Duration
	BackupInterval            time.Duration
	HealthCheckFailedCountMax int
	ReadyCheckFailedCountMax  int
	S3Client                  s3util.Client
	ClientTLSConfig           *tls.Config
	EtcdPodManifestWritePath  string
}

type Node struct {
	Name    string
	PeerURL string
}

func New() (*Args, error) {
	args := &Args{}

	// etcd args
	var initialAdvertisePeerURLs, listenPeerURLs, advertiseClientURLs, listenClientURLs, initialCluster string
	flag.StringVar(&args.Name, "name", "", "Human-readable name for this member.")
	flag.StringVar(&args.CertFile, "cert-file", "", "Host path to the client server TLS cert file.")
	flag.StringVar(&args.KeyFile, "key-file", "", "Host path to the client server TLS key file.")
	flag.StringVar(&args.TrustedCAFile, "trusted-ca-file", "", "Host path to the client server TLS trusted CA cert file.")
	flag.StringVar(&args.PeerCertFile, "peer-cert-file", "", "Host path to the peer server TLS cert file.")
	flag.StringVar(&args.PeerKeyFile, "peer-key-file", "", "Host path to the peer server TLS key file.")
	flag.StringVar(&args.PeerTrustedCAFile, "peer-trusted-ca-file", "", "Host path to the peer server TLS trusted CA file.")
	flag.StringVar(&args.InitialClusterToken, "initial-cluster-token", "", "Initial cluster token for the etcd cluster during bootstrap.")
	flag.StringVar(&initialAdvertisePeerURLs, "initial-advertise-peer-urls", "", "List of this member's peer URLs to advertise to the rest of the cluster.")
	flag.StringVar(&listenPeerURLs, "listen-peer-urls", "", "List of URLs to listen on for peer traffic.")
	flag.StringVar(&advertiseClientURLs, "advertise-client-urls", "", "List of this member's client URLs to advertise to the public.")
	flag.StringVar(&listenClientURLs, "listen-client-urls", "", "List of URLs to listen on for client traffic.")
	flag.StringVar(&initialCluster, "initial-cluster", "", "Initial cluster configuration for bootstrapping.")

	// etcd wrapper args
	var etcdPodTemplateFile, clientCertFile, clientKeyFile, s3BackupEndpoint, s3BackupResource string
	flag.StringVar(&etcdPodTemplateFile, "etcd-pod-template-file", "", "Path to the manifest template file for etcd pod.")
	flag.StringVar(&clientCertFile, "client-cert-file", "", "Path to the client server TLS cert file.")
	flag.StringVar(&clientKeyFile, "client-key-file", "", "Path to the client server TLS key file.")
	flag.StringVar(&args.EtcdSnapshotFile, "etcd-snaphot-file", "/var/lib/etcd/etcd.db", "Host path to restore snapshot file.")
	flag.StringVar(&args.EtcdPodManifestWritePath, "etcd-pod-manifest-write-path", "", "Host path to write etcd pod manifest file. This should be where kubelet reads static pod manifests.")
	flag.StringVar(&s3BackupEndpoint, "s3-backup-endpoint", "s3.amazonaws.com", "S3 endpoint for backup.")
	flag.StringVar(&s3BackupResource, "s3-backup-resource", "", "S3 resource name for backup.")
	flag.DurationVar(&args.HealthCheckInterval, "healthcheck-interval", 6*time.Second, "Healthcheck interval.")
	flag.DurationVar(&args.BackupInterval, "backup-interval", 15*time.Minute, "Backup trigger interval.")
	flag.IntVar(&args.HealthCheckFailedCountMax, "healthcheck-fail-count-allowed", 16, "Number of healthcheck failures to allow before restarting etcd pod.")
	flag.IntVar(&args.ReadyCheckFailedCountMax, "readiness-fail-count-allowed", 64, "Number of readiness check failures to allow before restarting etcd pod.")
	flag.Parse()

	var err error
	reList := regexp.MustCompile(`\s*,\s*`)
	reMap := regexp.MustCompile(`\s*=\s*`)

	args.EtcdPod, err = podFromManifest(etcdPodTemplateFile)
	if err != nil {
		return nil, err
	}
	args.S3Client, err = s3util.New(s3BackupEndpoint)
	if err != nil {
		return nil, err
	}
	args.S3BackupBucket, args.S3BackupKey, err = s3util.ParseBucketAndKey(s3BackupResource)
	if err != nil {
		return nil, err
	}
	args.ClientTLSConfig, err = transport.TLSInfo{
		CertFile:      clientCertFile,
		KeyFile:       clientKeyFile,
		TrustedCAFile: args.TrustedCAFile,
	}.ClientConfig()
	if err != nil {
		return nil, err
	}

	for _, i := range reList.Split(initialAdvertisePeerURLs, -1) {
		args.InitialAdvertisePeerURLs = append(args.InitialAdvertisePeerURLs, i)
	}
	for _, i := range reList.Split(advertiseClientURLs, -1) {
		args.AdvertiseClientURLs = append(args.AdvertiseClientURLs, i)
	}
	for _, i := range reList.Split(listenPeerURLs, -1) {
		args.ListenPeerURLs = append(args.ListenPeerURLs, i)
	}
	for _, i := range reList.Split(listenClientURLs, -1) {
		args.ListenClientURLs = append(args.ListenClientURLs, i)
	}
	for _, member := range reList.Split(initialCluster, -1) {
		k := reMap.Split(member, 2)
		node := &Node{
			Name:    k[0],
			PeerURL: k[1],
		}
		args.InitialCluster = append(args.InitialCluster, node)
	}
	return args, nil
}

func podFromManifest(templateFile string) (*v1.Pod, error) {
	manifest, err := os.ReadFile(templateFile)
	if err != nil {
		return nil, err
	}
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, gKV, err := decode(manifest, nil, nil)
	if err != nil {
		return nil, err
	}
	if gKV.Kind != "Pod" {
		return nil, fmt.Errorf("Etcd pod manifest template kind must be Pod.")
	}
	return obj.(*v1.Pod), nil
}
