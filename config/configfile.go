package config

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

type YamlConfig struct {
	CheckQuorumDelay  time.Duration `yaml:"checkQuorumDelay,omitempty"`
	ClientTimeout     time.Duration `yaml:"clusterTimeout,omitempty"`
	EtcdBinaryFile    string        `yaml:"etcdBinaryFile,omitempty"`
	S3Endpoint        string        `yaml:"s3Endpoint"`
	S3Region          string        `yaml:"s3Region,omitempty"`
	S3Bucket          string        `yaml:"s3Bucket"`
	S3TrustedCAs      []string      `yaml:"s3TrustedCAs,omitempty"`
	S3AccessKeyID     string        `yaml:"s3AccessKeyID"`
	S3SecretAccessKey string        `yaml:"s3SecretAccessKey"`
}

func (config *YamlConfig) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddDuration("CheckQuorumDelay", config.CheckQuorumDelay)
	enc.AddDuration("ClientTimeout", config.ClientTimeout)
	enc.AddString("EtcdBinaryFile", config.EtcdBinaryFile)
	return nil
}

func LoadConfigFile(path string) (*YamlConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file '%s': %w", path, err)
	}
	raw := &YamlConfig{
		S3Endpoint:       "https://s3.amazonaws.com",
		S3Region:         "us-east-1",
		CheckQuorumDelay: 1 * time.Minute,
		ClientTimeout:    8 * time.Second,
		EtcdBinaryFile:   "/usr/local/bin/etcd",
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

	return raw, nil
}
