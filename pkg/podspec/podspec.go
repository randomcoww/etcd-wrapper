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

func Create(args *arg.Args, runRestore bool) *v1.Pod {
	pod := args.EtcdPod.DeepCopy()
	pod.ObjectMeta.Name = fmt.Sprintf("%s-%s", pod.ObjectMeta.Name, args.InitialClusterState)
	etcdContainer := appendEtcdContainer(args, pod)
	appendTLS(args, pod, etcdContainer)
	appendCommonEnvs(args, pod, etcdContainer)

	if runRestore {
		restoreContainer := appendRestoreContainer(args, pod, etcdContainer)
		appendCommonEnvs(args, pod, restoreContainer)
	}
	return pod
}

func appendEtcdContainer(args *arg.Args, pod *v1.Pod) *v1.Container {
	for i, container := range pod.Spec.Containers {
		if container.Name == etcdContainerName {
			return &pod.Spec.Containers[i]
		}
	}
	pod.Spec.Containers = append(pod.Spec.Containers, v1.Container{
		Name: etcdContainerName,
	})
	return &pod.Spec.Containers[len(pod.Spec.Containers)-1]
}

func appendRestoreContainer(args *arg.Args, pod *v1.Pod, etcdContainer *v1.Container) *v1.Container {
	hostPathFileType := v1.HostPathFile

	pod.Spec.Volumes = append(pod.Spec.Volumes, []v1.Volume{
		{
			Name: fmt.Sprintf("db-%s", args.Name),
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{
					Medium: v1.StorageMediumMemory,
				},
			},
		},
		{
			Name: fmt.Sprintf("restore-%s", args.Name),
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: args.EtcdSnapshotFile,
					Type: &hostPathFileType,
				},
			},
		},
	}...)

	sharedVolumeMount := v1.VolumeMount{
		Name:      fmt.Sprintf("db-%s", args.Name),
		MountPath: dataDir,
	}
	etcdContainer.VolumeMounts = append(etcdContainer.VolumeMounts, sharedVolumeMount)
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, v1.Container{
		Name:  restoreContainerName,
		Image: etcdContainer.Image,
		Env: []v1.EnvVar{
			{
				Name:  "ETCDCTL_API",
				Value: "3",
			},
		},
		Command: strings.Split(fmt.Sprintf("etcdutl snapshot restore %s"+
			" --name $(ETCD_NAME)"+
			" --initial-cluster $(ETCD_INITIAL_CLUSTER)"+
			" --initial-cluster-token $(ETCD_INITIAL_CLUSTER_TOKEN)"+
			" --initial-advertise-peer-urls $(ETCD_INITIAL_ADVERTISE_PEER_URLS)"+
			" --data-dir $(ETCD_DATA_DIR)", args.EtcdSnapshotFile), " "),
		VolumeMounts: []v1.VolumeMount{
			sharedVolumeMount,
			{
				Name:      fmt.Sprintf("restore-%s", args.Name),
				MountPath: args.EtcdSnapshotFile,
			},
		},
	})
	return &pod.Spec.InitContainers[len(pod.Spec.InitContainers)-1]
}

func appendCommonEnvs(args *arg.Args, pod *v1.Pod, container *v1.Container) {
	var memberPeers []string
	for _, node := range args.InitialCluster {
		memberPeers = append(memberPeers, fmt.Sprintf("%s=%s", node.Name, node.PeerURL))
	}
	initialCluster := strings.Join(memberPeers, ",")
	container.Env = append(container.Env, []v1.EnvVar{
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
			Value: initialCluster,
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
	}...)
}

func appendTLS(args *arg.Args, pod *v1.Pod, container *v1.Container) {
	hostPathFileType := v1.HostPathFile

	for _, mount := range []struct {
		env  string
		path string
	}{
		{
			env:  "ETCD_CERT_FILE",
			path: args.CertFile,
		},
		{
			env:  "ETCD_KEY_FILE",
			path: args.KeyFile,
		},
		{
			env:  "ETCD_TRUSTED_CA_FILE",
			path: args.TrustedCAFile,
		},
		{
			env:  "ETCD_PEER_CERT_FILE",
			path: args.PeerCertFile,
		},
		{
			env:  "ETCD_PEER_KEY_FILE",
			path: args.PeerKeyFile,
		},
		{
			env:  "ETCD_PEER_TRUSTED_CA_FILE",
			path: args.PeerTrustedCAFile,
		},
	} {
		volumeName := strings.ToLower(strings.ReplaceAll(mount.env, "_", "-"))
		container.Env = append(container.Env, v1.EnvVar{
			Name:  mount.env,
			Value: mount.path,
		})
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: volumeName,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: mount.path,
					Type: &hostPathFileType,
				},
			},
		})
		container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			Name:      volumeName,
			MountPath: mount.path,
			ReadOnly:  true,
		})
	}
}
