package config

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

type YamlConfig struct {
	LocalClientURL        string        `yaml:"localClientURL"`
	ClientTimeout         time.Duration `yaml:"clusterTimeout,omitempty"`
	InitialClusterTimeout time.Duration `yaml:"initialClusterTimeout,omitempty"`
	EtcdBinaryFile        string        `yaml:"etcdBinaryFile"`
	S3Endpoint            string        `yaml:"s3Endpoint"`
	S3Region              string        `yaml:"s3Region,omitempty"`
	S3Bucket              string        `yaml:"s3Bucket"`
	S3TrustedCAs          []string      `yaml:"s3TrustedCAs,omitempty"`
	S3AccessKeyID         string        `yaml:"s3AccessKeyID"`
	S3SecretAccessKey     string        `yaml:"s3SecretAccessKey"`
}

func (config *YamlConfig) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("LocalClientURL", config.LocalClientURL)
	enc.AddDuration("InitialClusterTimeout", config.InitialClusterTimeout)
	enc.AddDuration("ClientTimeout", config.ClientTimeout)
	enc.AddString("EtcdBinaryFile", config.EtcdBinaryFile)
	return nil
}

func LoadConfigFile(path string) (*YamlConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	raw := &YamlConfig{
		S3Endpoint:            "https://s3.amazonaws.com",
		S3Region:              "us-east-1",
		ClientTimeout:         8 * time.Second,
		InitialClusterTimeout: 2 * time.Minute,
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// --- validate local client URL

	u, err := url.Parse(raw.LocalClientURL)
	if err != nil {
		return nil, fmt.Errorf("parse local client url: %w", err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("local client URL scheme must be HTTPS")
	}
	raw.LocalClientURL = fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	// --- validate etcd binary

	info, err := os.Stat(raw.EtcdBinaryFile)
	if err != nil {
		return nil, fmt.Errorf("stat etcd binary file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("etcd binary is not a valid file")
	}

	return raw, nil
}
