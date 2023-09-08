package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"strings"
)

const (
	// Shared path for etcd to read result of restored backup
	dataMountDir = "/var/etcd"
	// etcdctl snapshot restore can only be called on directory that doesn't exist
	dataDir = dataMountDir + "/data"
	// backup restore path
	backupFilePath = "/var/lib/etcd/data.db"
)

func makeRestoreInitContainer(m *Config) v1.Container {
	return v1.Container{
		Name:  "restore-datadir",
		Image: m.EtcdImage,
		Env: []v1.EnvVar{
			{
				Name:  "ETCDCTL_API",
				Value: "3",
			},
		},
		Command: strings.Split(fmt.Sprintf("etcdutl snapshot restore %[1]s"+
			" --name %[2]s"+
			" --initial-cluster %[3]s"+
			" --initial-cluster-token %[4]s"+
			" --initial-advertise-peer-urls %[5]s"+
			" --data-dir %[6]s",
			backupFilePath, m.Name, m.InitialCluster, m.InitialClusterToken, m.InitialAdvertisePeerURLs, dataDir), " "),
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "host-backup-file",
				MountPath: backupFilePath,
			},
			{
				Name:      "data-mount-path",
				MountPath: dataMountDir,
			},
		},
	}
}

func makeEtcdContainer(m *Config, state string) v1.Container {
	return v1.Container{
		Name:  "etcd",
		Image: m.EtcdImage,
		Env: []v1.EnvVar{
			{
				Name:  "ETCD_NAME",
				Value: m.Name,
			},
			{
				Name:  "ETCD_DATA_DIR",
				Value: dataDir,
			},
			{
				Name:  "ETCD_INITIAL_CLUSTER",
				Value: m.InitialCluster,
			},
			{
				Name:  "ETCD_INITIAL_CLUSTER_STATE",
				Value: state,
			},
			{
				Name:  "ETCD_INITIAL_CLUSTER_TOKEN",
				Value: m.InitialClusterToken,
			},
			{
				Name:  "ETCD_ENABLE_V2",
				Value: "false",
			},
			// Listen
			{
				Name:  "ETCD_LISTEN_CLIENT_URLS",
				Value: m.ListenClientURLs,
			},
			{
				Name:  "ETCD_ADVERTISE_CLIENT_URLS",
				Value: m.AdvertiseClientURLs,
			},
			{
				Name:  "ETCD_LISTEN_PEER_URLS",
				Value: m.ListenPeerURLs,
			},
			{
				Name:  "ETCD_INITIAL_ADVERTISE_PEER_URLS",
				Value: m.InitialAdvertisePeerURLs,
			},
			// TLS
			{
				Name:  "ETCD_PEER_CLIENT_CERT_AUTH",
				Value: "true",
			},
			{
				Name:  "ETCD_CLIENT_CERT_AUTH",
				Value: "true",
			},
			{
				Name:  "ETCD_CERT_FILE",
				Value: "/etc/etcd/cert.pem",
			},
			{
				Name:  "ETCD_KEY_FILE",
				Value: "/etc/etcd/key.pem",
			},
			{
				Name:  "ETCD_TRUSTED_CA_FILE",
				Value: "/etc/etcd/ca.pem",
			},
			{
				Name:  "ETCD_PEER_CERT_FILE",
				Value: "/etc/etcd/peer-cert.pem",
			},
			{
				Name:  "ETCD_PEER_KEY_FILE",
				Value: "/etc/etcd/peer-key.pem",
			},
			{
				Name:  "ETCD_PEER_TRUSTED_CA_FILE",
				Value: "/etc/etcd/peer-ca.pem",
			},
			{
				Name:  "ETCD_STRICT_RECONFIG_CHECK",
				Value: "true",
			},
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "host-cert-file",
				MountPath: "/etc/etcd/cert.pem",
				ReadOnly:  true,
			},
			{
				Name:      "host-key-file",
				MountPath: "/etc/etcd/key.pem",
				ReadOnly:  true,
			},
			{
				Name:      "host-trusted-ca-file",
				MountPath: "/etc/etcd/ca.pem",
				ReadOnly:  true,
			},
			{
				Name:      "host-peer-cert-file",
				MountPath: "/etc/etcd/peer-cert.pem",
				ReadOnly:  true,
			},
			{
				Name:      "host-peer-key-file",
				MountPath: "/etc/etcd/peer-key.pem",
				ReadOnly:  true,
			},
			{
				Name:      "host-peer-trusted-ca-file",
				MountPath: "/etc/etcd/peer-ca.pem",
				ReadOnly:  true,
			},
			{
				Name:      "data-mount-path",
				MountPath: dataMountDir,
			},
		},
	}
}

func NewEtcdPod(m *Config, state string, runRestore bool) *v1.Pod {
	hostPathFile := v1.HostPathFile
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        m.EtcdPodName,
			Namespace:   m.EtcdPodNamespace,
			Annotations: map[string]string{},
		},
		Spec: v1.PodSpec{
			HostNetwork:    true,
			InitContainers: []v1.Container{},
			Containers: []v1.Container{
				makeEtcdContainer(m, state),
			},
			RestartPolicy: v1.RestartPolicyAlways,
			Volumes: []v1.Volume{
				{
					Name: "host-cert-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.CertFile,
							Type: &hostPathFile,
						},
					},
				},
				{
					Name: "host-key-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.KeyFile,
							Type: &hostPathFile,
						},
					},
				},
				{
					Name: "host-trusted-ca-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.TrustedCAFile,
							Type: &hostPathFile,
						},
					},
				},
				{
					Name: "host-peer-cert-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.PeerCertFile,
							Type: &hostPathFile,
						},
					},
				},
				{
					Name: "host-peer-key-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.PeerKeyFile,
							Type: &hostPathFile,
						},
					},
				},
				{
					Name: "host-peer-trusted-ca-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.PeerTrustedCAFile,
							Type: &hostPathFile,
						},
					},
				},
				// Share restored DB with init-container
				{
					Name: "data-mount-path",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	// Run recovery for existing clusters
	if runRestore {
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			v1.Volume{
				Name: "host-backup-file",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: m.BackupFile,
						Type: &hostPathFile,
					},
				},
			},
		)

		pod.Spec.InitContainers = append(pod.Spec.InitContainers,
			makeRestoreInitContainer(m),
		)
	}

	return pod
}

func WritePodSpec(pod *v1.Pod, file string) error {
	j, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pod spec: %v", err)
	}

	err = os.MkdirAll(filepath.Dir(file), os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("failed to create base directory: %v", err)
	}

	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	_, err = io.Copy(f, bytes.NewReader(j))
	if err != nil {
		return fmt.Errorf("failed to restore snapshot file: %v", err)
	}
	return err
}
