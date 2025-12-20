package config

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/tlsutil"
	"go.uber.org/zap"
	"net"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Config struct {
	LocalClientURL           string
	InitialAdvertisePeerURLs []string
	ClusterPeerURLs          []string
	Env                      map[string]string
	ClientTLSConfig          *tls.Config
	PeerTLSConfig            *tls.Config
	Logger                   *zap.Logger
	EtcdBinaryFile           string
	EtcdutlBinaryFile        string
	S3BackupHost             string
	S3BackupBucket           string
	S3BackupKey              string
	S3TLSConfig              *tls.Config
	ClusterTimeout           time.Duration
	RestoreTimeout           time.Duration
	ReplaceTimeout           time.Duration
	UploadTimeout            time.Duration
	StatusTimeout            time.Duration
	NodeRunInterval          time.Duration
}

func NewConfig(args []string) (*Config, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	config := &Config{
		Env:    make(map[string]string),
		Logger: logger,
	}
	var s3resource, s3CAFile string
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.StringVar(&config.EtcdBinaryFile, "etcd-binary-file", config.EtcdBinaryFile, "Path to etcd binary")
	flags.StringVar(&config.EtcdutlBinaryFile, "etcdutl-binary-file", config.EtcdutlBinaryFile, "Path to etcdutl binary")
	flags.StringVar(&s3resource, "s3-backup-resource", s3resource, "S3 resource for backup")
	flags.StringVar(&s3CAFile, "s3-backup-ca-file", s3CAFile, "CA file for S3 resource")
	flags.DurationVar(&config.ClusterTimeout, "initial-cluster-timeout", 2*time.Minute, "Initial existing cluster lookup timeout")
	flags.DurationVar(&config.RestoreTimeout, "restore-snapshot-timeout", 1*time.Minute, "Restore snapshot timeout")
	flags.DurationVar(&config.ReplaceTimeout, "member-replace-timeout", 30*time.Second, "Member replace timeout")
	flags.DurationVar(&config.UploadTimeout, "backup-snapshot-timeout", 1*time.Minute, "Backup snapshot timeout")
	flags.DurationVar(&config.StatusTimeout, "status-timeout", 30*time.Second, "Local member status lookup timeout")
	flags.DurationVar(&config.NodeRunInterval, "node-run-interval", 15*time.Minute, "Node status check and backup interval")
	if err := flags.Parse(args[1:]); err != nil {
		return nil, err
	}
	if config.EtcdBinaryFile == "" {
		return nil, fmt.Errorf("etcd-binary-file not set")
	}
	if config.EtcdutlBinaryFile == "" {
		return nil, fmt.Errorf("etcdutl-binary-file not set")
	}

	u, err := url.Parse(s3resource)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, fmt.Errorf("host not found in s3-backup-resource")
	}
	config.S3BackupHost = u.Host
	parts := strings.Split(u.Path, "/")
	if len(parts) < 3 { // path always starts with / so first element should be blank
		return nil, fmt.Errorf("bucket and key not found in s3-backup-resource")
	}
	config.S3BackupBucket = parts[1]
	config.S3BackupKey = strings.Join(parts[2:], "/")

	var s3CAFiles []string
	if s3CAFile != "" {
		s3CAFiles = append(s3CAFiles, s3CAFile)
	}
	config.S3TLSConfig, err = tlsutil.TLSCAConfig(s3CAFiles)
	if err != nil {
		return nil, err
	}

	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "ETCD_") {
			k := strings.SplitN(e, "=", 2)
			config.Env[k[0]] = k[1]
		}
	}
	if err := config.ParseEnvs(); err != nil {
		return nil, err
	}

	return config, nil
}

func (config *Config) ParseEnvs() error {
	var err error
	reList := regexp.MustCompile(`\s*,\s*`)
	reMap := regexp.MustCompile(`\s*=\s*`)

	if v, ok := config.Env["ETCD_LISTEN_CLIENT_URLS"]; ok {
		config.LocalClientURL, err = getLocalURL(reList.Split(v, -1))
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("env ETCD_LISTEN_CLIENT_URLS not set")
	}

	if v, ok := config.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"]; ok {
		for _, u := range reList.Split(v, -1) {
			config.InitialAdvertisePeerURLs = append(config.InitialAdvertisePeerURLs, u)
		}
		sort.Strings(config.InitialAdvertisePeerURLs)
	} else {
		return fmt.Errorf("env ETCD_INITIAL_ADVERTISE_PEER_URLS not set")
	}

	if v, ok := config.Env["ETCD_INITIAL_CLUSTER"]; ok {
		for _, member := range reList.Split(v, -1) {
			k := reMap.Split(member, 2)
			config.ClusterPeerURLs = append(config.ClusterPeerURLs, k[1])
		}
	} else {
		return fmt.Errorf("env ETCD_INITIAL_CLUSTER not set")
	}

	if _, ok := config.Env["ETCD_DATA_DIR"]; !ok {
		return fmt.Errorf("env ETCD_DATA_DIR is not set")
	}

	config.Env["ETCD_CLIENT_CERT_AUTH"] = "true"
	trustedCAFile, ok := config.Env["ETCD_TRUSTED_CA_FILE"]
	if !ok {
		return fmt.Errorf("env ETCD_TRUSTED_CA_FILE is required")
	}
	certFile, ok := config.Env["ETCD_CERT_FILE"]
	if !ok {
		return fmt.Errorf("env ETCD_CERT_FILE is required")
	}
	keyFile, ok := config.Env["ETCD_KEY_FILE"]
	if !ok {
		return fmt.Errorf("env ETCD_KEY_FILE is required")
	}
	config.ClientTLSConfig, err = tlsutil.TLSConfig([]string{trustedCAFile}, certFile, keyFile)
	if err != nil {
		return err
	}

	config.Env["ETCD_PEER_CLIENT_CERT_AUTH"] = "true"
	peerTrustedCAFile, ok := config.Env["ETCD_PEER_TRUSTED_CA_FILE"]
	if !ok {
		return fmt.Errorf("env ETCD_PEER_TRUSTED_CA_FILE is required")
	}
	peerCertFile, ok := config.Env["ETCD_PEER_CERT_FILE"]
	if !ok {
		return fmt.Errorf("env ETCD_PEER_CERT_FILE is required")
	}
	peerKeyFile, ok := config.Env["ETCD_PEER_KEY_FILE"]
	if !ok {
		return fmt.Errorf("env ETCD_PEER_KEY_FILE is required")
	}
	config.PeerTLSConfig, err = tlsutil.TLSConfig([]string{peerTrustedCAFile}, peerCertFile, peerKeyFile)
	if err != nil {
		return err
	}

	delete(config.Env, "ETCD_INITIAL_CLUSTER_STATE") // this is set internally
	config.Env["ETCD_LOG_OUTPUTS"] = "stdout"
	config.Env["ETCD_ENABLE_V2"] = "false"
	config.Env["ETCD_STRICT_RECONFIG_CHECK"] = "true"
	config.Env["ETCDCTL_API"] = "3" // used by etcdutl
	return nil
}

func (config *Config) WriteEnv() []string {
	var envs []string
	for k, v := range config.Env {
		envs = append(envs, k+"="+v)
	}
	sort.Strings(envs)
	return envs
}

func getLocalURL(urls []string) (string, error) {
	var s string
	for _, s = range urls {
		u, err := url.Parse(s)
		if err != nil {
			return "", err
		}
		host, port, err := net.SplitHostPort(u.Host)
		if err != nil {
			return "", err
		}
		if port == "" {
			return "", fmt.Errorf("Port not found in listen client URL: %s", s)
		}
		if host == "localhost" {
			return s, nil
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return "", fmt.Errorf("IP not found in listen client URL: %s", s)
		}
		if ip.IsLoopback() {
			return s, nil
		}
		if ip.IsUnspecified() {
			return fmt.Sprintf("%s://%s:%s", u.Scheme, "localhost", port), nil
		}
	}
	return s, nil
}
