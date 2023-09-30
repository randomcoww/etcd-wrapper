package podspec

import (
	"fmt"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"strings"
)

const (
	dbFile               string = "/var/etcd/data"
	etcdContainerName    string = "etcd"
	restoreContainerName string = "etcd-snap-restore"
)

func Create(name, certFile, keyFile, trustedCAFile, peerCertFile, peerKeyFile, peerTrustedCAFile, initialAdvertisePeerURLs,
	listenPeerURLs, advertiseClientURLs, listenClientURLs, initialClusterToken, initialCluster string,
	etcdImage, etcdPodName, etcdPodNamespace, etcdSnapshotFile, etcdPodManifestFile string,
	initialClusterState string, runRestore bool, memberAnnotation uint64, versionAnnotation string) *v1.Pod {

	var priority int32 = 2000001000
	hostPathFileType := v1.HostPathFile
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdPodName,
			Namespace: etcdPodNamespace,
			Annotations: map[string]string{
				"etcd-wrapper/member":  fmt.Sprintf("%v", memberAnnotation),
				"etcd-wrapper/version": versionAnnotation,
			},
		},
		Spec: v1.PodSpec{
			HostNetwork:       true,
			InitContainers:    []v1.Container{},
			Containers:        []v1.Container{},
			PriorityClassName: "system-node-critical",
			Priority:          &priority,
			RestartPolicy:     v1.RestartPolicyAlways,
			Volumes: []v1.Volume{
				{
					Name: "cert-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: certFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "key-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: keyFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "trusted-ca-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: trustedCAFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "peer-cert-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: peerCertFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "peer-key-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: peerKeyFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "peer-trusted-ca-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: peerTrustedCAFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: fmt.Sprintf("db-%s", name),
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{
							Medium: v1.StorageMediumMemory,
						},
					},
				},
			},
		},
	}

	// Run recovery for existing clusters
	if runRestore {
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			v1.Volume{
				Name: "snaphot-restore",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: filepath.Dir(etcdSnapshotFile),
						Type: &hostPathFileType,
					},
				},
			},
		)

		pod.Spec.InitContainers = append(pod.Spec.InitContainers,
			v1.Container{
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
					etcdSnapshotFile, name, initialCluster, initialClusterToken, initialAdvertisePeerURLs, dbFile), " "),
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      fmt.Sprintf("db-%s", name),
						MountPath: filepath.Dir(dbFile),
					},
					{
						Name:      "snaphot-restore",
						MountPath: etcdSnapshotFile,
					},
				},
			},
		)
	}

	pod.Spec.Containers = append(pod.Spec.Containers,
		v1.Container{
			Name:  etcdContainerName,
			Image: etcdImage,
			Env: []v1.EnvVar{
				{
					Name:  "ETCD_NAME",
					Value: name,
				},
				{
					Name:  "ETCD_DATA_DIR",
					Value: dbFile,
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
					Name:      "cert-file",
					MountPath: "/etc/etcd/cert.pem",
					ReadOnly:  true,
				},
				{
					Name:      "key-file",
					MountPath: "/etc/etcd/key.pem",
					ReadOnly:  true,
				},
				{
					Name:      "trusted-ca-file",
					MountPath: "/etc/etcd/ca.pem",
					ReadOnly:  true,
				},
				{
					Name:      "peer-cert-file",
					MountPath: "/etc/etcd/peer-cert.pem",
					ReadOnly:  true,
				},
				{
					Name:      "peer-key-file",
					MountPath: "/etc/etcd/peer-key.pem",
					ReadOnly:  true,
				},
				{
					Name:      "peer-trusted-ca-file",
					MountPath: "/etc/etcd/peer-ca.pem",
					ReadOnly:  true,
				},
				{
					Name:      fmt.Sprintf("db-%s", name),
					MountPath: filepath.Dir(dbFile),
				},
			},
		},
	)

	return pod
}
