package config

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/randomcoww/etcd-wrapper/internal/tlsutil"
	"gopkg.in/yaml.v3"
)

type YamlConfig struct {
	CheckQuorumDelay       time.Duration `yaml:"checkQuorumDelay,omitempty"`
	RestoreSnapshotTimeout time.Duration `yaml:"restoreSnapshotTimeout,omitempty"`
	MemberTimeout          time.Duration `yaml:"memberTimeout,omitempty"`
	EtcdBinaryFile         string        `yaml:"etcdBinaryFile,omitempty"`
	S3Endpoint             string        `yaml:"s3Endpoint"`
	S3Region               string        `yaml:"s3Region,omitempty"`
	S3Bucket               string        `yaml:"s3Bucket"`
	S3TrustedCAs           []string      `yaml:"s3TrustedCAs,omitempty"`
	S3AccessKeyID          string        `yaml:"s3AccessKeyID"`
	S3SecretAccessKey      string        `yaml:"s3SecretAccessKey"`
	S3TLSConfig            *tls.Config
	SnapshotObjectPrefix   string            `yaml:"snapshotObjectPrefix"`
	SnapshotEncryptionKey  string            `yaml:"snapshotEncryptionKey"`
	GossipPeerURLs         map[string]string `yaml:"gossipPeerURLs"`
}

func LoadConfigFile(path string) (*YamlConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file '%s': %w", path, err)
	}
	raw := &YamlConfig{
		S3Endpoint:             "https://s3.amazonaws.com",
		S3Region:               "us-east-1",
		CheckQuorumDelay:       16 * time.Second,
		MemberTimeout:          8 * time.Second,
		RestoreSnapshotTimeout: 8 * time.Second,
		SnapshotObjectPrefix:   "etcd-snapshot-",
		EtcdBinaryFile:         "/usr/local/bin/etcd",
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	info, err := os.Stat(raw.EtcdBinaryFile)
	if err != nil {
		return nil, fmt.Errorf("stat etcd binary file '%s': %w", raw.EtcdBinaryFile, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("etcd binary file '%s' is not a regular file", raw.EtcdBinaryFile)
	}

	if raw.S3Bucket == "" {
		return nil, fmt.Errorf("missing s3Bucket")
	}
	u, err := url.Parse(raw.S3Endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse advertise url: %w", err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("S3 URL scheme must be HTTPS")
	}
	raw.S3Endpoint = u.Host

	raw.S3TLSConfig, err = tlsutil.BuildTLSCAConfig(raw.S3TrustedCAs)
	if err != nil {
		return nil, fmt.Errorf("building S3 TLS config: %w", err)
	}

	for node, peerURL := range raw.GossipPeerURLs {
		u, err := url.Parse(peerURL)
		if err != nil {
			return nil, fmt.Errorf("parse peer URL: %w", err)
		}
		raw.GossipPeerURLs[node] = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	}
	return raw, nil
}
