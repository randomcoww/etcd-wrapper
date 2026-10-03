package config

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/randomcoww/etcd-wrapper/internal/tlsutil"
	"go.uber.org/zap/zapcore"
)

type EnvConfig struct {
	Env                      map[string]string
	InitialAdvertisePeerURLs []string
	ClusterPeerURLs          []string
	ClientTLSConfig          *tls.Config
	PeerTLSConfig            *tls.Config
}

func (config *EnvConfig) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("Name", config.Env["ETCD_NAME"])
	enc.AddString("DataDir", config.Env["ETCD_DATA_DIR"])
	enc.AddString("WalDir", config.Env["ETCD_WAL_DIR"])
	enc.AddString("InitialAdvertisePeerURLs", fmt.Sprintf("%v", config.InitialAdvertisePeerURLs))
	enc.AddString("ClusterPeerURLs", fmt.Sprintf("%v", config.ClusterPeerURLs))
	return nil
}

func LoadFromEnv() (*EnvConfig, error) {
	var (
		err    error
		ok     bool
		reList = regexp.MustCompile(`\s*,\s*`)
		reMap  = regexp.MustCompile(`\s*=\s*`)
	)

	config := &EnvConfig{
		Env: make(map[string]string),
	}

	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "ETCD_") {
			k := strings.SplitN(e, "=", 2)
			config.Env[k[0]] = k[1]
		}
	}

	if _, ok := config.Env["ETCD_NAME"]; !ok {
		return nil, fmt.Errorf("env ETCD_NAME is not set")
	}
	if _, ok := config.Env["ETCD_INITIAL_CLUSTER_TOKEN"]; !ok {
		return nil, fmt.Errorf("env ETCD_INITIAL_CLUSTER_TOKEN is not set")
	}
	if _, ok := config.Env["ETCD_DATA_DIR"]; !ok {
		return nil, fmt.Errorf("env ETCD_DATA_DIR is not set")
	}

	initialCluster, ok := config.Env["ETCD_INITIAL_CLUSTER"]
	if !ok {
		return nil, fmt.Errorf("env ETCD_INITIAL_CLUSTER not set")
	}
	for _, member := range reList.Split(initialCluster, -1) {
		k := reMap.Split(member, 2)
		u, err := url.Parse(k[1])
		if err != nil {
			return nil, fmt.Errorf("parse initial cluster peer url: %w", err)
		}
		config.ClusterPeerURLs = append(config.ClusterPeerURLs, fmt.Sprintf("%s://%s", u.Scheme, u.Host))
	}

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
	config.PeerTLSConfig, err = tlsutil.BuildTLSClientConfig(peerCertFile, peerKeyFile, []string{peerTrustedCAFile})
	if err != nil {
		return nil, err
	}

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
	config.ClientTLSConfig, err = tlsutil.BuildTLSClientConfig(certFile, keyFile, []string{trustedCAFile})
	if err != nil {
		return nil, err
	}

	if v, ok := config.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"]; ok {
		for _, member := range reList.Split(v, -1) {
			u, err := url.Parse(member)
			if err != nil {
				return nil, fmt.Errorf("parse initial advertise peer url: %w", err)
			}
			config.InitialAdvertisePeerURLs = append(config.InitialAdvertisePeerURLs, fmt.Sprintf("%s://%s", u.Scheme, u.Host))
		}
		sort.Strings(config.InitialAdvertisePeerURLs)
	} else {
		return nil, fmt.Errorf("env ETCD_INITIAL_ADVERTISE_PEER_URLS not set")
	}

	delete(config.Env, "ETCD_INITIAL_CLUSTER_STATE") // this is set internally
	config.Env["ETCD_LOG_OUTPUTS"] = "stdout"
	config.Env["ETCD_ENABLE_V2"] = "false"
	config.Env["ETCD_STRICT_RECONFIG_CHECK"] = "true"
	config.Env["ETCD_CLIENT_CERT_AUTH"] = "true"
	config.Env["ETCD_PEER_CLIENT_CERT_AUTH"] = "true"

	return config, nil
}

func (config *EnvConfig) WriteEnv() []string {
	var envs []string
	for k, v := range config.Env {
		envs = append(envs, k+"="+v)
	}
	sort.Strings(envs)
	return envs
}
