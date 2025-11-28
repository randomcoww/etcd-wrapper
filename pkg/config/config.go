package config

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/tlsutil"
	"go.uber.org/zap"
	"os"
	"regexp"
	"sort"
	"strings"
)

type Config struct {
	ListenClientURLs         []string
	InitialAdvertisePeerURLs []string
	ClusterPeerURLs          []string
	Env                      map[string]string
	ClientTLSConfig          *tls.Config
	PeerTLSConfig            *tls.Config
	Logger                   *zap.Logger
	EtcdBinaryFile           string
	EtcdctlBinaryFile        string
}

func NewConfig() (*Config, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	config := &Config{
		Env:    make(map[string]string),
		Logger: logger,
	}
	flag.StringVar(&config.EtcdBinaryFile, "etcd-binary-file", config.EtcdBinaryFile, "Path to etcd binary")
	flag.StringVar(&config.EtcdctlBinaryFile, "etcdctl-binary-file", config.EtcdctlBinaryFile, "Path to etcdctl binary")
	flag.Parse()

	reList := regexp.MustCompile(`\s*,\s*`)
	reMap := regexp.MustCompile(`\s*=\s*`)

	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "ETCD_") {
			k := reMap.Split(e, 2)
			config.Env[k[0]] = k[1]
		}
	}

	if v, ok := config.Env["ETCD_LISTEN_CLIENT_URLS"]; ok {
		for _, u := range reList.Split(v, -1) {
			config.ListenClientURLs = append(config.ListenClientURLs, u)
		}
	} else {
		return nil, fmt.Errorf("env ETCD_LISTEN_CLIENT_URLS is not set")
	}

	if v, ok := config.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"]; ok {
		for _, u := range reList.Split(v, -1) {
			config.InitialAdvertisePeerURLs = append(config.InitialAdvertisePeerURLs, u)
		}
	} else {
		return nil, fmt.Errorf("env ETCD_INITIAL_ADVERTISE_PEER_URLS is not set")
	}

	if v, ok := config.Env["ETCD_INITIAL_CLUSTER"]; ok {
		for _, member := range reList.Split(v, -1) {
			k := reMap.Split(member, 2)
			config.ClusterPeerURLs = append(config.ClusterPeerURLs, k[1])
		}
	} else {
		return nil, fmt.Errorf("env ETCD_INITIAL_CLUSTER is not set")
	}

	config.Env["ETCD_CLIENT_CERT_AUTH"] = "true"
	trustedCAFile, ok := config.Env["ETCD_TRUSTED_CA_FILE"]
	if !ok {
		return nil, fmt.Errorf("env ETCD_TRUSTED_CA_FILE is required")
	}
	certFile, ok := config.Env["ETCD_CERT_FILE"]
	if !ok {
		return nil, fmt.Errorf("env ETCD_CERT_FILE is required")
	}
	keyFile, ok := config.Env["ETCD_KEY_FILE"]
	if !ok {
		return nil, fmt.Errorf("env ETCD_KEY_FILE is required")
	}
	config.ClientTLSConfig, err = tlsutil.TLSConfig(trustedCAFile, certFile, keyFile)
	if err != nil {
		return nil, err
	}

	config.Env["ETCD_PEER_CLIENT_CERT_AUTH"] = "true"
	peerTrustedCAFile, ok := config.Env["ETCD_PEER_TRUSTED_CA_FILE"]
	if !ok {
		return nil, fmt.Errorf("env ETCD_PEER_TRUSTED_CA_FILE is required")
	}
	peerCertFile, ok := config.Env["ETCD_PEER_CERT_FILE"]
	if !ok {
		return nil, fmt.Errorf("env ETCD_PEER_CERT_FILE is required")
	}
	peerKeyFile, ok := config.Env["ETCD_PEER_KEY_FILE"]
	if !ok {
		return nil, fmt.Errorf("env ETCD_PEER_KEY_FILE is required")
	}
	config.PeerTLSConfig, err = tlsutil.TLSConfig(peerTrustedCAFile, peerCertFile, peerKeyFile)
	if err != nil {
		return nil, err
	}

	config.Env["ETCD_LOG_OUTPUTS"] = "stdout"
	config.Env["ETCD_ENABLE_V2"] = "false"
	config.Env["ETCD_STRICT_RECONFIG_CHECK"] = "true"
	config.Env["ETCDCTL_API"] = "3"

	return config, nil
}

func (config *Config) WriteEnv() []string {
	var envs []string
	for k, v := range config.Env {
		envs = append(envs, k+"="+v)
	}
	sort.Strings(envs)
	return envs
}
