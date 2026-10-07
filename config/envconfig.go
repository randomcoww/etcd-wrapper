package config

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/randomcoww/etcd-wrapper/internal/tlsutil"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.uber.org/zap/zapcore"
)

type EnvConfig struct {
	Env                      map[string]string
	InitialAdvertisePeerURLs []string
	ListenClientURLs         []string
	ClusterPeerURLs          []string
	InitialCluster           map[string][]string
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
		err error
		ok  bool
	)

	config := &EnvConfig{
		Env:            make(map[string]string),
		InitialCluster: make(map[string][]string),
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
	if _, ok := config.Env["ETCD_ADVERTISE_CLIENT_URLS"]; !ok {
		return nil, fmt.Errorf("env ETCD_ADVERTISE_CLIENT_URLS is not set")
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

	if v, ok := config.Env["ETCD_INITIAL_CLUSTER"]; ok {
		var node string
		for _, p := range strings.Split(v, ",") {
			var peerURL string
			c := strings.Split(p, "=")
			switch len(c) {
			case 2:
				node = c[0]
				peerURL = c[1]
			case 1:
				peerURL = c[0]
			default:
				return nil, fmt.Errorf("malformed initial cluster")
			}
			if node == "" {
				return nil, fmt.Errorf("malformed initial cluster")
			}
			u, err := url.Parse(peerURL)
			if err != nil {
				return nil, fmt.Errorf("parse initial cluster peer url: %w", err)
			}
			config.InitialCluster[node] = append(config.InitialCluster[node], fmt.Sprintf("%s://%s", u.Scheme, u.Host))
			config.ClusterPeerURLs = append(config.ClusterPeerURLs, fmt.Sprintf("%s://%s", u.Scheme, u.Host))
		}
	} else {
		return nil, fmt.Errorf("env ETCD_INITIAL_CLUSTER not set")
	}

	if v, ok := config.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"]; ok {
		for _, member := range strings.Split(v, ",") {
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

	if v, ok := config.Env["ETCD_LISTEN_CLIENT_URLS"]; ok {
		for _, member := range strings.Split(v, ",") {
			u, err := url.Parse(member)
			if err != nil {
				return nil, fmt.Errorf("parse listen client url: %w", err)
			}
			config.ListenClientURLs = append(config.ListenClientURLs, fmt.Sprintf("%s://%s", u.Scheme, u.Host))
		}
		sort.Strings(config.ListenClientURLs)
	} else {
		return nil, fmt.Errorf("env ETCD_LISTEN_CLIENT_URLS is not set")
	}

	delete(config.Env, "ETCD_INITIAL_CLUSTER_STATE") // this is set internally
	config.Env["ETCD_LOG_OUTPUTS"] = "stdout"
	config.Env["ETCD_STRICT_RECONFIG_CHECK"] = "true"
	config.Env["ETCD_CLIENT_CERT_AUTH"] = "true"
	config.Env["ETCD_PEER_CLIENT_CERT_AUTH"] = "true"
	config.Env["ETCD_SOCKET_REUSE_ADDRESS"] = "true"
	config.Env["ETCD_SOCKET_REUSE_PORT"] = "true"

	return config, nil
}

func (config *EnvConfig) WriteEnv() []string {
	var initialClusterParts []string
	for name, peerURLs := range config.InitialCluster {
		initialClusterParts = append(initialClusterParts, fmt.Sprintf("%s=%s", name, strings.Join(peerURLs, ",")))
	}
	config.Env["ETCD_INITIAL_CLUSTER"] = strings.Join(initialClusterParts, ",")
	config.Env["ETCD_INITIAL_ADVERTISE_PEER_URLS"] = strings.Join(config.InitialAdvertisePeerURLs, ",")

	var envs []string
	for k, v := range config.Env {
		envs = append(envs, k+"="+v)
	}
	sort.Strings(envs)
	return envs
}

func (config *EnvConfig) RestoreConfig(snapshotPath string, revisionBump uint64) snapshot.RestoreConfig {
	r := snapshot.RestoreConfig{
		Name:                config.Env["ETCD_NAME"],
		OutputDataDir:       config.Env["ETCD_DATA_DIR"],
		OutputWALDir:        config.Env["ETCD_WAL_DIR"],
		PeerURLs:            config.InitialAdvertisePeerURLs,
		InitialCluster:      config.Env["ETCD_INITIAL_CLUSTER"],
		InitialClusterToken: config.Env["ETCD_INITIAL_CLUSTER_TOKEN"],
		RevisionBump:        revisionBump,
		SnapshotPath:        snapshotPath,
	}
	if r.RevisionBump > 0 {
		r.MarkCompacted = true
	}
	return r
}
