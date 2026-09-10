package config

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/tlsutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Env                      map[string]string
	Logger                   *zap.Logger
	LocalClientURL           string
	InitialAdvertisePeerURLs []string
	ClusterPeerURLs          []string
	ClientTLSConfig          *tls.Config
	PeerTLSConfig            *tls.Config
	EtcdBinaryFile           string
	InitialClusterTimeout    time.Duration
	ClientTimeout            time.Duration
}

func (config *Config) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("Name", config.Env["ETCD_NAME"])
	enc.AddString("DataDir", config.Env["ETCD_DATA_DIR"])
	enc.AddString("LocalClientURL", config.LocalClientURL)
	enc.AddString("InitialAdvertisePeerURLs", fmt.Sprintf("%v", config.InitialAdvertisePeerURLs))
	enc.AddString("ClusterPeerURLs", fmt.Sprintf("%v", config.ClusterPeerURLs))
	enc.AddString("EtcdBinaryFile", config.EtcdBinaryFile)
	enc.AddDuration("InitialClusterTimeout", config.InitialClusterTimeout)
	enc.AddDuration("ClientTimeout", config.ClientTimeout)
	return nil
}

func NewConfig(args []string) (*Config, error) {
	config := &Config{
		Env: make(map[string]string),
	}
	if err := config.parseArgs(args); err != nil {
		return nil, err
	}
	return config, nil
}

func (config *Config) parseArgs(args []string) error {
	var (
		err error
		ok  bool
		cmd string

		reList = regexp.MustCompile(`\s*,\s*`)
		reMap  = regexp.MustCompile(`\s*=\s*`)
	)

	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "ETCD_") {
			k := strings.SplitN(e, "=", 2)
			config.Env[k[0]] = k[1]
		}
	}

	if len(args) > 1 {
		cmd, args = args[0], args[1:]
	} else {
		return fmt.Errorf("not enough arguments")
	}

	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.StringVar(&config.LocalClientURL, "local-client-url", "", "URL of local etcd client")
	fs.DurationVar(&config.ClientTimeout, "client-timeout", 8*time.Second, "Client operations timeout")
	fs.DurationVar(&config.InitialClusterTimeout, "initial-cluster-timeout", 2*time.Minute, "Initial cluster discovery timeout")
	fs.StringVar(&config.EtcdBinaryFile, "etcd-binary-file", "/usr/local/bin/etcd", "Path to etcd binary")
	if err := fs.Parse(args); err != nil {
		return err
	}

	delete(config.Env, "ETCD_INITIAL_CLUSTER_STATE") // this is set internally

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
