package podspec

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
	restoreMountPath string = "/var/etcd"
	restoreDataPath string = restoreMountPath + "/data"
	backupFilePath string = "/var/lib/etcd/data.db"
	etcdContainerName string = "etcd"
	restoreContainerName string = etcdContainerName + "-snap-restore"
)

func WriteManifest(name, certFile, keyFile, trustedCAFile, peerCertFile, peerKeyFile, peerTrustedCAFile, initialAdvertisePeerURLs,
	listenPeerURLs, advertiseClientURLs, listenClientURLs, initialClusterToken, initialCluster string,
	etcdPodName, etcdPodNamespace, etcdImage, snapRestoreFile, podManifestFile string,
	initialClusterState string, snapRestore bool, memberAnnotation uint64) error {

	restoreContainerSpec := v1.Container{
		Name:  restoreContainerName,
		Image: etcdImage,
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
			backupFilePath, name, initialCluster, initialClusterToken, initialAdvertisePeerURLs, dataDir), " "),
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "host-backup-file",
				MountPath: backupFilePath,
			},
			{
				Name:      "snap-mount-path",
				MountPath: snapMountDir,
			},
		},
	}

	etcdContainerSpec := v1.Container{
		Name: etcdContainerName,
		Image: etcdImage,
		Env: []v1.EnvVar{
			{
				Name:  "ETCD_NAME",
				Value: name,
			},
			{
				Name:  "ETCD_DATA_DIR",
				Value: dataDir,
			},
			{
				Name:  "ETCD_INITIAL_CLUSTER",
				Value: initialCluster,
			},
			{
				Name:  "ETCD_INITIAL_CLUSTER_STATE",
				Value: initialClusterState,
			},
			{
				Name:  "ETCD_INITIAL_CLUSTER_TOKEN",
				Value: initialClusterToken,
			},
			{
				Name:  "ETCD_ENABLE_V2",
				Value: "false",
			},
			// Listen
			{
				Name:  "ETCD_LISTEN_CLIENT_URLS",
				Value: listenClientURLs,
			},
			{
				Name:  "ETCD_ADVERTISE_CLIENT_URLS",
				Value: advertiseClientURLs,
			},
			{
				Name:  "ETCD_LISTEN_PEER_URLS",
				Value: listenPeerURLs,
			},
			{
				Name:  "ETCD_INITIAL_ADVERTISE_PEER_URLS",
				Value: initialAdvertisePeerURLs,
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
				MountPath: restoreMountPath,
			},
		},
	}

	hostPathFileType := v1.HostPathFile
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        etcdPodName,
			Namespace:   etcdPodNamespace,
			Annotations: map[string]string{
				"etcd-wrapper/member": fmt.Sprintf("%s", memberAnnotation),
			},
		},
		Spec: v1.PodSpec{
			HostNetwork:    true,
			InitContainers: []v1.Container{},
			Containers: []v1.Container{
				createEtcdContainerSpec(),
			},
			RestartPolicy: v1.RestartPolicyAlways,
			Volumes: []v1.Volume{
				{
					Name: "host-cert-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.CertFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "host-key-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.KeyFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "host-trusted-ca-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.TrustedCAFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "host-peer-cert-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.PeerCertFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "host-peer-key-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.PeerKeyFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "host-peer-trusted-ca-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: m.PeerTrustedCAFile,
							Type: &hostPathFileType,
						},
					},
				},
				// Share restored DB with init-container
				{
					Name: "restore-mount-path",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
	
	// Run recovery for existing clusters
	if snapRestore {
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			v1.Volume{
				Name: "host-snap-restore-file",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: backupFile,
						Type: &hostPathFileType,
					},
				},
			},
		)

		pod.Spec.InitContainers = append(pod.Spec.InitContainers,
			createRestoreContainerSpec(),
		)
	}

	return writePodManifest(pod, manifestFile)
}

func writePodManifest(pod *v1.Pod, file string) error {
	manifest, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(file), os.FileMode(0644))
	if err != nil {
		return err
	}

	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, bytes.NewReader(manifest))
	if err != nil {
		return err
	}
	return nil
}