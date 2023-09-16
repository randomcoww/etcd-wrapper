### Overview

_"I want [etcd-operator](https://github.com/coreos/etcd-operator) like management for an etcd cluster, but without the dependecy on Kubernetes, which in turn depends on another etcd cluster"_

This is intended for managing a static on-prem etcd cluster bootstrapped with fixed peer and client URLs. Nodes are expected to come back on the same IP address if rebooted.

Each member node should be running (masterless) Kubelet with a `--pod-manifest-path` configured. Etcd-wrapper will write a static pod manifest for etcd to this path to start and update the etcd instance on the node.

There is no cleanup of etcd data on recovery steps which require removing old data. Data is intended to live in the etcd container and be discarded on pod restart.

### Workflow

- If no cluster exists, recover or start new etcd cluster.
  - Check backup location (S3), and attempt to recover.
  - If backup exists, recover existing cluster.
  - If backup can't be accessed, start new cluster.
  
- If a cluster exists but a member missing, start a new member to join the existing cluster.
  - Start a new etcd instance as an existing member.
  - If a conflicting old member is found in the cluster, remove it using etcd API.
  - Add missing member using etcd API.
  
![etcd-wrapper](images/etcd-wrapper.png)

- Periodic snapshot sent to S3 bucket.

### Sample etcd-wrapper deployed as a static pod

https://github.com/randomcoww/terraform-infra/blob/master/modules/template/kubernetes/templates/ignition_controller/controller.yaml#L182

### Image build

```
TAG=ghcr.io/randomcoww/etcd-wrapper:$(date -u +'%Y%m%d')

podman build \
  -t $TAG . && \

podman push $TAG
```

### Env

```
podman run -it --rm \
  -v $(pwd):/go/etcd-wrapper \
  -w /go/etcd-wrapper \
   golang:alpine sh
```

### Test cluster

```bash
IMAGE=gcr.io/etcd-development/etcd:v3.5.8-amd64
TOKEN=my-etcd-token-1
CLUSTER_STATE=new
NAME_1=etcd-1
NAME_2=etcd-2
NAME_3=etcd-3
HOST_PEER_1=127.0.0.1:40001
HOST_PEER_2=127.0.0.1:40002
HOST_PEER_3=127.0.0.1:40003
HOST_CLIENT_1=127.0.0.1:40004
HOST_CLIENT_2=127.0.0.1:40005
HOST_CLIENT_3=127.0.0.1:40006
CLUSTER=${NAME_1}=http://${HOST_PEER_1},${NAME_2}=http://${HOST_PEER_2},${NAME_3}=http://${HOST_PEER_3}

THIS_NAME=${NAME_1}
PEER=${HOST_PEER_1}
CLIENT=${HOST_CLIENT_1}
podman run -it --rm --name ${THIS_NAME} -e ETCDCTL_API=3 --net host ${IMAGE} \
  etcd \
  --data-dir=data.etcd \
  --name ${THIS_NAME} \
	--initial-advertise-peer-urls http://${PEER} \
	--listen-peer-urls http://${PEER} \
	--advertise-client-urls http://${CLIENT},http://127.0.0.1:40011 \
	--listen-client-urls http://${CLIENT} \
	--initial-cluster ${CLUSTER} \
	--initial-cluster-state ${CLUSTER_STATE} \
	--initial-cluster-token ${TOKEN}

# For node 2
THIS_NAME=${NAME_2}
PEER=${HOST_PEER_2}
CLIENT=${HOST_CLIENT_2}
podman run -it --rm --name ${THIS_NAME} -e ETCDCTL_API=3 --net host ${IMAGE} \
  etcd \
  --data-dir=data.etcd \
  --name ${THIS_NAME} \
	--initial-advertise-peer-urls http://${PEER} \
	--listen-peer-urls http://${PEER} \
	--advertise-client-urls http://${CLIENT} \
	--listen-client-urls http://${CLIENT} \
	--initial-cluster ${CLUSTER} \
	--initial-cluster-state ${CLUSTER_STATE} \
	--initial-cluster-token ${TOKEN}

# For node 3
THIS_NAME=${NAME_3}
PEER=${HOST_PEER_3}
CLIENT=${HOST_CLIENT_3}
podman run -it --rm --name ${THIS_NAME} -e ETCDCTL_API=3 --net host ${IMAGE} \
  etcd \
  --data-dir=data.etcd \
  --name ${THIS_NAME} \
	--initial-advertise-peer-urls http://${PEER} \
	--listen-peer-urls http://${PEER} \
	--advertise-client-urls http://${CLIENT} \
	--listen-client-urls http://${CLIENT} \
	--initial-cluster ${CLUSTER} \
	--initial-cluster-state ${CLUSTER_STATE} \
	--initial-cluster-token ${TOKEN}
```