package podspec

import (
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"k8s.io/api/core/v1"
	"strings"
)

const (
	etcdContainerName    string = "etcd"
	restoreContainerName string = "snapshot-restore"
	dataDir              string = "/var/etcd/data"
)

type pathEnv struct {
	name string
	path string
}

func Create(args *arg.Args, runRestore bool) (*v1.Pod, error) {
	pod := args.EtcdPod.DeepCopy()
	etcdContainer, err := etcdContainerFromPod(pod, etcdContainerName)
	if err != nil {
		return nil, err
	}
	pod.ObjectMeta.Name = fmt.Sprintf("%s-%s", pod.ObjectMeta.Name, args.InitialClusterState)

	dataVolumeName := fmt.Sprintf("db-%s", args.Name)
	pod.Spec.Volumes = append(pod.Spec.Volumes, []v1.Volume{
		{
			Name: dataVolumeName,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		},
	}...)

	// common envs
	var memberPeers []string
	for _, node := range args.InitialCluster {
		memberPeers = append(memberPeers, fmt.Sprintf("%s=%s", node.Name, node.PeerURL))
	}
	commonEnvs := []v1.EnvVar{
		{
			Name:  "ETCD_NAME",
			Value: args.Name,
		},
		{
			Name:  "ETCD_DATA_DIR",
			Value: dataDir,
		},
		{
			Name:  "ETCD_INITIAL_CLUSTER_TOKEN",
			Value: args.InitialClusterToken,
		},
		{
			Name:  "ETCD_INITIAL_CLUSTER_STATE",
			Value: args.InitialClusterState,
		},
		{
			Name:  "ETCD_INITIAL_CLUSTER",
			Value: strings.Join(memberPeers, ","),
		},
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
	}
	commonVolumeMounts := []v1.VolumeMount{
		{
			Name:      dataVolumeName,
			MountPath: dataDir,
		},
	}

	// etcd container
	etcdContainer.Env = append(etcdContainer.Env, commonEnvs...)
	etcdContainer.VolumeMounts = append(etcdContainer.VolumeMounts, commonVolumeMounts...)
	appendHostPathEnvs(pod, []*v1.Container{etcdContainer}, []*pathEnv{
		{
			name: "ETCD_CERT_FILE",
			path: args.CertFile,
		},
		{
			name: "ETCD_KEY_FILE",
			path: args.KeyFile,
		},
		{
			name: "ETCD_TRUSTED_CA_FILE",
			path: args.TrustedCAFile,
		},
		{
			name: "ETCD_PEER_CERT_FILE",
			path: args.PeerCertFile,
		},
		{
			name: "ETCD_PEER_KEY_FILE",
			path: args.PeerKeyFile,
		},
		{
			name: "ETCD_PEER_TRUSTED_CA_FILE",
			path: args.PeerTrustedCAFile,
		},
	})

	if runRestore {
		restoreContainer := v1.Container{
			Name:  restoreContainerName,
			Image: etcdContainer.Image,
			Env:   commonEnvs,
			Command: strings.Split("etcdutl snapshot restore $(ETCD_SNAPSHOT_FILE)"+
				" --name $(ETCD_NAME)"+
				" --initial-cluster $(ETCD_INITIAL_CLUSTER)"+
				" --initial-cluster-token $(ETCD_INITIAL_CLUSTER_TOKEN)"+
				" --initial-advertise-peer-urls $(ETCD_INITIAL_ADVERTISE_PEER_URLS)"+
				" --data-dir $(ETCD_DATA_DIR)", " "),
			VolumeMounts: commonVolumeMounts,
		}
		appendHostPathEnvs(pod, []*v1.Container{&restoreContainer}, []*pathEnv{
			{
				name: "ETCD_SNAPSHOT_FILE",
				path: args.EtcdSnapshotFile,
			},
		})
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, restoreContainer)
	}
	return pod, nil
}

func etcdContainerFromPod(pod *v1.Pod, name string) (*v1.Container, error) {
	for i, container := range pod.Spec.Containers {
		if container.Name == name {
			return &pod.Spec.Containers[i], nil
		}
	}
	return nil, fmt.Errorf("Container '%s' not found in etcd pod manifest template.", name)
}

func appendHostPathEnvs(pod *v1.Pod, containers []*v1.Container, pathEnvs []*pathEnv) {
	hostPathFileType := v1.HostPathFile
	for _, env := range pathEnvs {
		volumeName := strings.ToLower(strings.ReplaceAll(env.name, "_", "-"))
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: volumeName,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: env.path,
					Type: &hostPathFileType,
				},
			},
		})
		for _, container := range containers {
			container.Env = append(container.Env, v1.EnvVar{
				Name:  env.name,
				Value: env.path,
			})
			container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
				Name:      volumeName,
				MountPath: env.path,
			})
		}
	}
}
