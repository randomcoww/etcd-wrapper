package podspec

import (
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"strings"
)

const (
	etcdContainerName    string = "etcd"
	restoreContainerName string = "snapshot-restore"
	dbFile               string = "/var/etcd/data"
)

func Create(args *arg.Args, runRestore bool, versionAnnotation string) *v1.Pod {
	var priority int32 = 2000001000
	var memberPeers []string
	for _, node := range args.InitialCluster {
		memberPeers = append(memberPeers, fmt.Sprintf("%s=%s", node.Name, node.PeerURL))
	}
	initialCluster := strings.Join(memberPeers, ",")

	hostPathFileType := v1.HostPathFile
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.EtcdPodName,
			Namespace: args.EtcdPodNamespace,
			Annotations: map[string]string{
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
							Path: args.CertFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "key-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: args.KeyFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "trusted-ca-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: args.TrustedCAFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "peer-cert-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: args.PeerCertFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "peer-key-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: args.PeerKeyFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: "peer-trusted-ca-file",
					VolumeSource: v1.VolumeSource{
						HostPath: &v1.HostPathVolumeSource{
							Path: args.PeerTrustedCAFile,
							Type: &hostPathFileType,
						},
					},
				},
				{
					Name: fmt.Sprintf("db-%s", args.Name),
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
				Name: "snapshot-restore",
				VolumeSource: v1.VolumeSource{
					HostPath: &v1.HostPathVolumeSource{
						Path: args.EtcdSnapshotFile,
						Type: &hostPathFileType,
					},
				},
			},
		)

		pod.Spec.InitContainers = append(pod.Spec.InitContainers,
			v1.Container{
				Name:  restoreContainerName,
				Image: args.EtcdImage,
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
					args.EtcdSnapshotFile, args.Name, initialCluster, args.InitialClusterToken, strings.Join(args.InitialAdvertisePeerURLs, ","), dbFile), " "),
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      fmt.Sprintf("db-%s", args.Name),
						MountPath: filepath.Dir(dbFile),
					},
					{
						Name:      "snapshot-restore",
						MountPath: args.EtcdSnapshotFile,
					},
				},
			},
		)
	}

	pod.Spec.Containers = append(pod.Spec.Containers,
		v1.Container{
			Name:  etcdContainerName,
			Image: args.EtcdImage,
			Env: []v1.EnvVar{
				{
					Name:  "ETCD_NAME",
					Value: args.Name,
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
					Value: args.InitialClusterState,
				},
				{
					Name:  "ETCD_INITIAL_CLUSTER_TOKEN",
					Value: args.InitialClusterToken,
				},
				{
					Name:  "ETCD_ENABLE_V2",
					Value: "false",
				},
				{
					Name:  "ETCD_STRICT_RECONFIG_CHECK",
					Value: "true",
				},
				{
					Name:  "ETCD_AUTO_COMPACTION_RETENTION",
					Value: args.AutoCompationRetention,
				},
				{
					Name:  "ETCD_AUTO_COMPACTION_MODE",
					Value: "revision",
				},
				// Listen
				{
					Name:  "ETCD_LISTEN_CLIENT_URLS",
					Value: strings.Join(args.ListenClientURLs, ","),
				},
				{
					Name:  "ETCD_ADVERTISE_CLIENT_URLS",
					Value: strings.Join(args.AdvertiseClientURLs, ","),
				},
				{
					Name:  "ETCD_LISTEN_PEER_URLS",
					Value: strings.Join(args.ListenPeerURLs, ","),
				},
				{
					Name:  "ETCD_INITIAL_ADVERTISE_PEER_URLS",
					Value: strings.Join(args.InitialAdvertisePeerURLs, ","),
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
					Name:      fmt.Sprintf("db-%s", args.Name),
					MountPath: filepath.Dir(dbFile),
				},
			},
		},
	)

	return pod
}
