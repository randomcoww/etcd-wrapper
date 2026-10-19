package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type YamlConfig struct {
	CheckQuorumDelay time.Duration `yaml:"checkQuorumDelay,omitempty"`
	MemberTimeout    time.Duration `yaml:"memberTimeout,omitempty"`
	EtcdBinaryFile   string        `yaml:"etcdBinaryFile,omitempty"`

	S3Endpoint             string        `yaml:"s3Endpoint,omitempty"`
	S3Region               string        `yaml:"s3Region,omitempty"`
	S3Bucket               string        `yaml:"s3Bucket"`
	S3TrustedCAs           []string      `yaml:"s3TrustedCAs,omitempty"`
	S3AccessKeyID          string        `yaml:"s3AccessKeyID"`
	S3SecretAccessKey      string        `yaml:"s3SecretAccessKey"`
	SnapshotEncryptionKey  string        `yaml:"snapshotEncryptionKey"`
	SnapshotObjectPrefix   string        `yaml:"snapshotObjectPrefix,omitempty"`
	SnapshotRetentionCount int           `yaml:"snapshotRetentionCount,omitempty"`
	S3ClientTimeout        time.Duration `yaml:"s3ClientTimeout,omitempty"`
	RestoreSnapshotTimeout time.Duration `yaml:"restoreSnapshotTimeout,omitempty"`
	UploadSnapshotTimeout  time.Duration `yaml:"uploadSnapshotTimeout,omitempty"`
	BackupInterval         time.Duration `yaml:"backupInterval,omitempty"`
	SnapshotTimeout        time.Duration `yaml:"snapshotTimeout,omitempty"`
	DefragmentTimeout      time.Duration `yaml:"defragmentTimeout,omitempty"`
	DefragThreshold        float64       `yaml:"defragThreshold,omitempty"`
}

func LoadConfigFile(path string) (*YamlConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file '%s': %w", path, err)
	}
	raw := &YamlConfig{
		CheckQuorumDelay:       2 * time.Minute,
		MemberTimeout:          4 * time.Second,
		EtcdBinaryFile:         "/usr/local/bin/etcd",
		S3Endpoint:             "https://s3.amazonaws.com",
		S3Region:               "us-east-1",
		SnapshotObjectPrefix:   "etcd-snapshot-",
		SnapshotRetentionCount: 4,
		S3ClientTimeout:        8 * time.Second,
		RestoreSnapshotTimeout: 30 * time.Second,
		UploadSnapshotTimeout:  30 * time.Second,
		BackupInterval:         5 * time.Minute,
		SnapshotTimeout:        30 * time.Second,
		DefragmentTimeout:      30 * time.Second,
		DefragThreshold:        0.3,
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return raw, nil
}
