package config

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/tlsutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Cmd                      string
	Env                      map[string]string
	Logger                   *zap.Logger
	LocalClientURL           string
	InitialAdvertisePeerURLs []string
	ClusterPeerURLs          []string
	ClientTLSConfig          *tls.Config
	PeerTLSConfig            *tls.Config
	EtcdBinaryFile           string
	EtcdutlBinaryFile        string
	S3BackupHost             string
	S3BackupBucket           string
	S3BackupKeyPrefix        string
	S3BackupCount            int
	S3VerifyTimeout          time.Duration
	S3TLSConfig              *tls.Config
	InitialClusterTimeout    time.Duration
	RestoreTimeout           time.Duration
	ClientTimeout            time.Duration
	UploadTimeout            time.Duration
	BackupInterval           time.Duration
}

func (config *Config) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("Name", config.Env["ETCD_NAME"])
	enc.AddString("DataDir", config.Env["ETCD_DATA_DIR"])
	enc.AddString("LocalClientURL", config.LocalClientURL)
	enc.AddString("InitialAdvertisePeerURLs", fmt.Sprintf("%v", config.InitialAdvertisePeerURLs))
	enc.AddString("ClusterPeerURLs", fmt.Sprintf("%v", config.ClusterPeerURLs))
	enc.AddString("EtcdBinaryFile", config.EtcdBinaryFile)
	enc.AddString("EtcdutlBinaryFile", config.EtcdutlBinaryFile)
	enc.AddString("S3BackupHost", config.S3BackupHost)
	enc.AddString("S3BackupBucket", config.S3BackupBucket)
	enc.AddString("S3BackupKeyPrefix", config.S3BackupKeyPrefix)
	enc.AddInt("S3BackupCount", config.S3BackupCount)
	enc.AddDuration("S3VerifyTimeout", config.S3VerifyTimeout)
	enc.AddDuration("InitialClusterTimeout", config.InitialClusterTimeout)
	enc.AddDuration("RestoreTimeout", config.RestoreTimeout)
	enc.AddDuration("ClientTimeout", config.ClientTimeout)
	enc.AddDuration("UploadTimeout", config.UploadTimeout)
	enc.AddDuration("BackupInterval", config.BackupInterval)
	return nil
}

func NewConfig(cmd string, args []string) (*Config, error) {
	config := &Config{
		Cmd: cmd,
		Env: make(map[string]string),
	}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "ETCD_") {
			k := strings.SplitN(e, "=", 2)
			config.Env[k[0]] = k[1]
		}
	}
	if err := config.ParseArgs(args); err != nil {
		return nil, err
	}
	return config, nil
}

func (config *Config) ParseArgs(args []string) error {
	var (
		s3Resource, s3CAFile string
		err                  error
		ok                   bool
	)
	reList := regexp.MustCompile(`\s*,\s*`)
	reMap := regexp.MustCompile(`\s*=\s*`)

	fs := flag.NewFlagSet(config.Cmd, flag.ExitOnError)
	fs.StringVar(&config.LocalClientURL, "local-client-url", config.LocalClientURL, "URL of local etcd client")
	fs.StringVar(&s3Resource, "s3-backup-resource-prefix", s3Resource, "S3 resource prefix for backup")
	fs.StringVar(&s3CAFile, "s3-backup-trusted-ca-file", s3CAFile, "Custom CA for internal S3")
	fs.StringVar(&config.EtcdutlBinaryFile, "etcdutl-binary-file", "/usr/local/bin/etcdutl", "Path to etcdutl binary")
	fs.DurationVar(&config.ClientTimeout, "client-timeout", 8*time.Second, "Client operations timeout")
	fs.DurationVar(&config.S3VerifyTimeout, "s3-verify-timeout", 10*time.Second, "S3 backup access verify timeout")

	switch config.Cmd {
	case "run":
		fs.DurationVar(&config.InitialClusterTimeout, "initial-cluster-timeout", 2*time.Minute, "Initial cluster discovery timeout")
		fs.StringVar(&config.EtcdBinaryFile, "etcd-binary-file", "/usr/local/bin/etcd", "Path to etcd binary")
		fs.DurationVar(&config.RestoreTimeout, "restore-snapshot-timeout", 1*time.Minute, "Restore snapshot timeout")
	case "backup":
		fs.DurationVar(&config.UploadTimeout, "upload-snapshot-timeout", 1*time.Minute, "Upload snapshot timeout")
		fs.DurationVar(&config.BackupInterval, "backup-interval", 10*time.Minute, "Backup interval")
		fs.IntVar(&config.S3BackupCount, "s3-backup-count", 4, "count of snapshots to retain")
	default:
		return fmt.Errorf("unsupported command %s", config.Cmd)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	u, err := url.Parse(s3Resource)
	if err != nil {
		return err
	}
	if u.Host == "" {
		return fmt.Errorf("host not found in s3-backup-resource")
	}
	config.S3BackupHost = u.Host
	parts := strings.Split(u.Path, "/")
	if len(parts) < 3 { // path always starts with / so first element should be blank
		return fmt.Errorf("bucket and key not found in s3-backup-resource-prefix")
	}
	config.S3BackupBucket = parts[1]
	config.S3BackupKeyPrefix = strings.Join(parts[2:], "/")

	var s3CAFiles []string
	if s3CAFile != "" {
		s3CAFiles = append(s3CAFiles, s3CAFile)
	}
	config.S3TLSConfig, err = tlsutil.TLSCAConfig(s3CAFiles)
	if err != nil {
		return err
	}
	delete(config.Env, "ETCD_INITIAL_CLUSTER_STATE") // this is set internally
	delete(config.Env, "ETCD_WAL_DIR")               // simplify with just ETCD_DATA_DIR

	config.Env["ETCDCTL_API"] = "3" // used by etcdutl

	if v, ok := config.Env["ETCD_INITIAL_CLUSTER"]; ok {
		for _, member := range reList.Split(v, -1) {
			k := reMap.Split(member, 2)
			config.ClusterPeerURLs = append(config.ClusterPeerURLs, k[1])
		}
	} else {
		return fmt.Errorf("env ETCD_INITIAL_CLUSTER not set")
	}

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

	switch config.Cmd {
	case "run":
		if _, ok := config.Env["ETCD_NAME"]; !ok {
			return fmt.Errorf("env ETCD_NAME is not set")
		}
		if v, ok := config.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"]; ok {
			config.InitialAdvertisePeerURLs = append(config.InitialAdvertisePeerURLs, reList.Split(v, -1)...)
			sort.Strings(config.InitialAdvertisePeerURLs)
		} else {
			return fmt.Errorf("env ETCD_INITIAL_ADVERTISE_PEER_URLS not set")
		}
		if _, ok := config.Env["ETCD_DATA_DIR"]; !ok {
			return fmt.Errorf("env ETCD_DATA_DIR is not set")
		}

		config.Env["ETCD_LOG_OUTPUTS"] = "stdout"
		config.Env["ETCD_ENABLE_V2"] = "false"
		config.Env["ETCD_STRICT_RECONFIG_CHECK"] = "true"
		config.Env["ETCD_CLIENT_CERT_AUTH"] = "true"
		config.Env["ETCD_PEER_CLIENT_CERT_AUTH"] = "true"
	}
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
