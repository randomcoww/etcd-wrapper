package podspec

import (
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"testing"
)

func TestPodSpec(t *testing.T) {
	int32Ptr := func(i int32) *int32 {
		v := i
		return &v
	}

	hostPathFileType := v1.HostPathFile
	tests := []struct {
		label           string
		args            *arg.Args
		runRestore      bool
		expectedPodSpec *v1.Pod
	}{
		{
			label: "new etcd pod with restore",
			args: &arg.Args{
				Name:             "etcd-name",
				EtcdPodName:      "etcd-pod",
				EtcdPodNamespace: "test-ns",
				EtcdImage:        "etcd-image:latest",
				EtcdPodLabels: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
				InitialCluster: []*arg.Node{
					&arg.Node{
						Name:    "node0",
						PeerURL: "https://10.0.0.1:8001",
					},
					&arg.Node{
						Name:    "node1",
						PeerURL: "https://10.0.0.2:8001",
					},
					&arg.Node{
						Name:    "node2",
						PeerURL: "https://10.0.0.3:8001",
					},
				},
				InitialClusterState: "new",
				InitialClusterToken: "etcd-token",
				ListenClientURLs: []string{
					"https://10.0.0.1:8001",
					"https://10.0.0.2:8001",
					"https://10.0.0.3:8001",
				},
				AdvertiseClientURLs: []string{
					"https://10.0.0.1:8001",
					"https://10.0.0.2:8001",
					"https://10.0.0.3:8001",
				},
				ListenPeerURLs: []string{
					"https://10.0.0.1:8002",
					"https://10.0.0.2:8002",
					"https://10.0.0.3:8002",
				},
				InitialAdvertisePeerURLs: []string{
					"https://10.0.0.1:8002",
					"https://10.0.0.2:8002",
					"https://10.0.0.3:8002",
				},
				AutoCompationRetention: "0",
				PodPriorityClassName:   "system-cluster-critical",
				PodPriority:            2000000000,
				CertFile:               "/etc/etcd/cert.pem",
				KeyFile:                "/etc/etcd/key.pem",
				TrustedCAFile:          "/etc/etcd/ca-cert.pem",
				PeerCertFile:           "/etc/etcd/peer-cert.pem",
				PeerKeyFile:            "/etc/etcd/peer-key.pem",
				PeerTrustedCAFile:      "/etc/etcd/peer-ca-cert.pem",
				EtcdSnapshotFile:       "/var/lib/etcd/snapshot.db",
			},
			runRestore: true,
			expectedPodSpec: &v1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "etcd-pod-new",
					Namespace: "test-ns",
					Labels: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
				Spec: v1.PodSpec{
					HostNetwork: true,
					InitContainers: []v1.Container{
						v1.Container{
							Name:  "snapshot-restore",
							Image: "etcd-image:latest",
							Env: []v1.EnvVar{
								{
									Name:  "ETCDCTL_API",
									Value: "3",
								},
							},
							Command: strings.Split("etcdutl snapshot restore /var/lib/etcd/snapshot.db"+
								" --name etcd-name"+
								" --initial-cluster node0=https://10.0.0.1:8001,node1=https://10.0.0.2:8001,node2=https://10.0.0.3:8001"+
								" --initial-cluster-token etcd-token"+
								" --initial-advertise-peer-urls https://10.0.0.1:8002,https://10.0.0.2:8002,https://10.0.0.3:8002"+
								" --data-dir /var/etcd/data", " "),
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "db-etcd-name",
									MountPath: "/var/etcd/data",
								},
								{
									Name:      "snapshot-restore",
									MountPath: "/var/lib/etcd/snapshot.db",
								},
							},
						},
					},
					Containers: []v1.Container{
						v1.Container{
							Name:  "etcd",
							Image: "etcd-image:latest",
							Env: []v1.EnvVar{
								{
									Name:  "ETCD_NAME",
									Value: "etcd-name",
								},
								{
									Name:  "ETCD_DATA_DIR",
									Value: "/var/etcd/data",
								},
								{
									Name:  "ETCD_INITIAL_CLUSTER",
									Value: "node0=https://10.0.0.1:8001,node1=https://10.0.0.2:8001,node2=https://10.0.0.3:8001",
								},
								{
									Name:  "ETCD_INITIAL_CLUSTER_STATE",
									Value: "new",
								},
								{
									Name:  "ETCD_INITIAL_CLUSTER_TOKEN",
									Value: "etcd-token",
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
									Value: "0",
								},
								{
									Name:  "ETCD_AUTO_COMPACTION_MODE",
									Value: "revision",
								},
								// Listen
								{
									Name:  "ETCD_LISTEN_CLIENT_URLS",
									Value: "https://10.0.0.1:8001,https://10.0.0.2:8001,https://10.0.0.3:8001",
								},
								{
									Name:  "ETCD_ADVERTISE_CLIENT_URLS",
									Value: "https://10.0.0.1:8001,https://10.0.0.2:8001,https://10.0.0.3:8001",
								},
								{
									Name:  "ETCD_LISTEN_PEER_URLS",
									Value: "https://10.0.0.1:8002,https://10.0.0.2:8002,https://10.0.0.3:8002",
								},
								{
									Name:  "ETCD_INITIAL_ADVERTISE_PEER_URLS",
									Value: "https://10.0.0.1:8002,https://10.0.0.2:8002,https://10.0.0.3:8002",
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
									Name:      "db-etcd-name",
									MountPath: "/var/etcd/data",
								},
							},
						},
					},
					PriorityClassName: "system-cluster-critical",
					Priority:          int32Ptr(2000000000),
					RestartPolicy:     v1.RestartPolicyAlways,
					Volumes: []v1.Volume{
						{
							Name: "cert-file",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/etc/etcd/cert.pem",
									Type: &hostPathFileType,
								},
							},
						},
						{
							Name: "key-file",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/etc/etcd/key.pem",
									Type: &hostPathFileType,
								},
							},
						},
						{
							Name: "trusted-ca-file",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/etc/etcd/ca-cert.pem",
									Type: &hostPathFileType,
								},
							},
						},
						{
							Name: "peer-cert-file",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/etc/etcd/peer-cert.pem",
									Type: &hostPathFileType,
								},
							},
						},
						{
							Name: "peer-key-file",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/etc/etcd/peer-key.pem",
									Type: &hostPathFileType,
								},
							},
						},
						{
							Name: "peer-trusted-ca-file",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/etc/etcd/peer-ca-cert.pem",
									Type: &hostPathFileType,
								},
							},
						},
						{
							Name: "db-etcd-name",
							VolumeSource: v1.VolumeSource{
								EmptyDir: &v1.EmptyDirVolumeSource{
									Medium: v1.StorageMediumMemory,
								},
							},
						},
						{
							Name: "snapshot-restore",
							VolumeSource: v1.VolumeSource{
								HostPath: &v1.HostPathVolumeSource{
									Path: "/var/lib/etcd/snapshot.db",
									Type: &hostPathFileType,
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {

			podSpec := Create(tt.args, tt.runRestore)
			assert.Equal(t, tt.expectedPodSpec, podSpec)
		})
	}
}
