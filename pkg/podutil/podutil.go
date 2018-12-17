package podutil

import (
	"fmt"
	"strings"
	// "io/ioutil"
	"io"
	"bytes"
	"os"
	"encoding/json"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dataDir = "/var/etcd/data"
)

type Spec struct {
	Name string

	// Mount this to run etcdctl snapshot restore
	BackupMountDir string
	// This path should be under BackupMountDir
	BackupFile string

	// Mount this in etcd container - cert files should all be under this path
	EtcdTLSMountDir string
	// These paths should be under EtcdTLSMountDir
	CertFile string
	KeyFile string
	TrustedCAFile string
	PeerCertFile string
	PeerKeyFile string
	PeerTrustedCAFile string

	// Listen
	InitialAdvertisePeerURLs string
	ListenPeerURLs string
	AdvertiseClientURLs string
	ListenClientURLs string

	InitialClusterToken string
	InitialCluster string

	// etcd image
	Repository string
	Version string
	// kubelet static pod path
	PodSpecFile string
	S3BackupPath string
}

func ClientURLs(m *Spec) []string {
	return strings.Split(m.ListenClientURLs, ",")
}

func makeRestoreInitContainer(m *Spec) v1.Container {
	return v1.Container{
		Name:  "restore-datadir",
		Image: m.Repository + ":" + m.Version,
		Env: []v1.EnvVar{
			{
				Name: "ETCDCTL_API",
				Value: "3",
			},
		},
		Command: []string{
			fmt.Sprintf("/usr/local/bin/etcdctl snapshot restore %[1]s" +
				" --name %[2]s" +
				" --initial-cluster %[3]s" +
				" --initial-cluster-token %[4]s" +
				" --initial-advertise-peer-urls %[5]s" +
				" --data-dir %[6]s",
				m.BackupFile, m.Name, m.InitialCluster, m.InitialClusterToken, m.InitialAdvertisePeerURLs, dataDir),
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name: "backup-path", 
				MountPath: m.BackupMountDir,
			},
		},
	}
}

func makeEtcdContainer(m *Spec, state string) v1.Container {
	return v1.Container{
		Name:  "etcd",
		Image: m.Repository + ":" + m.Version,
		Env: []v1.EnvVar{
			{
				Name: "ETCD_NAME",
				Value: m.Name,
			},
			{
				Name: "ETCD_DATA_DIR",
				Value: dataDir,
			},
			{
				Name: "ETCD_INITIAL_CLUSTER",
				Value: m.InitialCluster,
			},
			{
				Name: "ETCD_INITIAL_CLUSTER_STATE",
				Value: state,
			},
			{
				Name: "ETCD_INITIAL_CLUSTER_TOKEN",
				Value: m.InitialClusterToken,
			},
			{
				Name: "ETCD_ENABLE_V2",
				Value: "false",
			},
			// Listen
			{
				Name: "ETCD_LISTEN_CLIENT_URLS",
				Value: m.ListenClientURLs,
			},
			{
				Name: "ETCD_ADVERTISE_CLIENT_URLS",
				Value: m.AdvertiseClientURLs,
			},				
			{
				Name: "ETCD_LISTEN_PEER_URLS",
				Value: m.ListenPeerURLs,
			},
			{
				Name: "ETCD_INITIAL_ADVERTISE_PEER_URLS",
				Value: m.InitialAdvertisePeerURLs,
			},
			// TLS
			{
				Name: "ETCD_PEER_CLIENT_CERT_AUTH",
				Value: "true",
			},
			{
				Name: "ETCD_CLIENT_CERT_AUTH",
				Value: "true",
			},
			{
				Name: "ETCD_CERT_FILE",
				Value: m.CertFile,
			},
			{
				Name: "ETCD_KEY_FILE",
				Value: m.KeyFile,
			},
			{
				Name: "ETCD_TRUSTED_CA_FILE",
				Value: m.TrustedCAFile,
			},
			{
				Name: "ETCD_PEER_CERT_FILE",
				Value: m.PeerCertFile,
			},
			{
				Name: "ETCD_PEER_KEY_FILE",
				Value: m.PeerKeyFile,
			},
			{
				Name: "ETCD_PEER_TRUSTED_CA_FILE",
				Value: m.PeerTrustedCAFile,
			},
		},
		Command: []string{
			"/usr/local/bin/etcd",
		},
		VolumeMounts: []v1.VolumeMount{
			{
				Name: "cert-path", 
				MountPath: m.EtcdTLSMountDir,
			},
		},
	}
}

func NewEtcdPod(m *Spec, state string, runRestore bool) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: m.Name,
		},
		Spec: v1.PodSpec{
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