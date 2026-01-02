package config

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/tlsutil"
	"go.uber.org/zap"
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
	StatusTimeout            time.Duration
	UploadTimeout            time.Duration
	Cmd                      string
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

	flag.Usage = func() {
		fmt.Printf(`Usage:

	etcd-wrapper <command> [arguments]

Commands:

	run       Check existing cluster and run etcd as new or existing
	backup    Run periodic backup
`)
	}

	if len(args) > 0 {
		config.Cmd, args = args[0], args[1:]
	}

	switch config.Cmd {
	case "run":
		fs := flag.NewFlagSet(config.Cmd, flag.ExitOnError)

		fs.StringVar(&config.EtcdBinaryFile, "etcd-binary-file", config.EtcdBinaryFile, "Path to etcd binary")
		fs.StringVar(&config.EtcdutlBinaryFile, "etcdutl-binary-file", config.EtcdutlBinaryFile, "Path to etcdutl binary")
		fs.DurationVar(&config.RestoreTimeout, "restore-snapshot-timeout", 1*time.Minute, "Restore snapshot timeout")
		fs.DurationVar(&config.ReplaceTimeout, "member-replace-timeout", 30*time.Second, "Member replace timeout")

		if err := config.parseWithCommonArgs(fs, args); err != nil {
			return nil, err
		}
		if config.EtcdBinaryFile == "" {
			return nil, fmt.Errorf("etcd-binary-file not set")
		}
		if config.EtcdutlBinaryFile == "" {
			return nil, fmt.Errorf("etcdutl-binary-file not set")
		}

		for _, e := range os.Environ() {
			if strings.HasPrefix(e, "ETCD_") {
				k := strings.SplitN(e, "=", 2)
				config.Env[k[0]] = k[1]
			}
		}
		if err := config.parseEnvs(); err != nil {
			return nil, err
		}

	case "backup":
		fs := flag.NewFlagSet(config.Cmd, flag.ExitOnError)

		fs.DurationVar(&config.StatusTimeout, "status-timeout", 30*time.Second, "Member status timeout")
		fs.DurationVar(&config.UploadTimeout, "upload-snapshot-timeout", 1*time.Minute, "Upload snapshot timeout")

		if err := config.parseWithCommonArgs(fs, args); err != nil {
			return nil, err
		}

	default:
		flag.Usage()
	}
	return config, nil
}

func (config *Config) parseWithCommonArgs(fs *flag.FlagSet, args []string) error {
	var s3resource, s3CAFile string
	fs.StringVar(&config.LocalClientURL, "local-client-url", config.LocalClientURL, "URL of local etcd client")
	fs.StringVar(&s3resource, "s3-backup-resource", s3resource, "S3 resource for backup")
	fs.StringVar(&s3CAFile, "s3-backup-ca-file", s3CAFile, "CA file for S3 resource")
	fs.DurationVar(&config.ClusterTimeout, "initial-cluster-timeout", 2*time.Minute, "Initial existing cluster lookup timeout")

	if err := fs.Parse(args); err != nil {
		return err
	}
	u, err := url.Parse(s3resource)
	if err != nil {
		return err
	}
	if u.Host == "" {
		return fmt.Errorf("host not found in s3-backup-resource")
	}
	config.S3BackupHost = u.Host
	parts := strings.Split(u.Path, "/")
	if len(parts) < 3 { // path always starts with / so first element should be blank
		return fmt.Errorf("bucket and key not found in s3-backup-resource")
	}
	config.S3BackupBucket = parts[1]
	config.S3BackupKey = strings.Join(parts[2:], "/")

	var s3CAFiles []string
	if s3CAFile != "" {
		s3CAFiles = append(s3CAFiles, s3CAFile)
	}
	config.S3TLSConfig, err = tlsutil.TLSCAConfig(s3CAFiles)
	if err != nil {
		return err
	}
	return nil
}

func (config *Config) parseEnvs() error {
	var err error
	reList := regexp.MustCompile(`\s*,\s*`)
	reMap := regexp.MustCompile(`\s*=\s*`)

	if _, ok := config.Env["ETCD_NAME"]; !ok {
		return fmt.Errorf("env ETCD_NAME is not set")
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
