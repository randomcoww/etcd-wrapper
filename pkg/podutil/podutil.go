package podutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/randomcoww/etcd-wrapper/pkg/cluster"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dataMountDir = "/var/etcd"
	dataDir = "/var/etcd/data"
)

func makeRestoreInitContainer(m *cluster.Cluster) v1.Container {
	return v1.Container{
		Name:  "restore-datadir",
		Image: m.Image,
		Env: []v1.EnvVar{
			{
				Name:  "ETCDCTL_API",
				Value: "3",
			},
		},
		Command: strings.Split(fmt.Sprintf("/usr/local/bin/etcdctl snapshot restore %[1]s"+
			" --name %[2]s"+
			" --initial-cluster %[3]s"+
			" --initial-cluster-token %[4]s"+
			" --initial-advertise-peer-urls %[5]s"+
			" --data-dir %[6]s",
			m.BackupFile, m.Name, m.InitialCluster, m.InitialClusterToken, m.InitialAdvertisePeerURLs, dataDir), " "),
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "backup-path",
				MountPath: m.BackupMountDir,
			},
			{
				Name:      "data-mount-path",
				MountPath: dataMountDir,
			},
		},
	}
}

func makeEtcdContainer(m *cluster.Cluster, state string) v1.Container {
	return v1.Container{
		Name:  "etcd",
		Image: m.Image,
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
				Value: m.CertFile,
			},
			{
				Name:  "ETCD_KEY_FILE",
				Value: m.KeyFile,
			},
			{
				Name:  "ETCD_TRUSTED_CA_FILE",
				Value: m.TrustedCAFile,
			},
			{
				Name:  "ETCD_PEER_CERT_FILE",
				Value: m.PeerCertFile,
			},
			{
				Name:  "ETCD_PEER_KEY_FILE",
				Value: m.PeerKeyFile,
			},
			{
				Name:  "ETCD_PEER_TRUSTED_CA_FILE",
				Value: m.PeerTrustedCAFile,
			},
		},
		Command: []string{
			"/usr/local/bin/etcd",
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "cert-path",
				MountPath: m.EtcdTLSMountDir,
			},
			{
				Name:      "data-mount-path",
				MountPath: dataMountDir,
			},
		},
	}
}

func NewEtcdPod(m *cluster.Cluster, state string, runRestore bool) *v1.Pod {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: m.Name,
			Annotations: map[string]string{
				"etcd-wrapper/instance": m.Instance,
			},
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
					Name: "backup-path",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.BackupMountDir,
						},
					},
				},
				{
					Name: "cert-path",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.EtcdTLSMountDir,
						},
					},
				},
				{
					Name: "data-mount-path",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{
						},
					},
				},
			},
			Hostname: m.Name,
		},
	}

	// Run recovery for existing clusters
	if runRestore {
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
		return fmt.Errorf("failed to create base directory: (%v)", err)
	}

	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: (%v)", err)
	}

	_, err = io.Copy(f, bytes.NewReader(j))
	if err != nil {
		return fmt.Errorf("failed to restore snapshot file: %v", err)
	}
	return err
}
