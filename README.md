#### Overview

_"I want [etcd-operator](https://github.com/coreos/etcd-operator) like management for an etcd cluster, but without the dependecy on Kubernetes, which in turn depends on another etcd cluster"_

This is intended for managing a static on-prem etcd cluster bootstrapped with fixed peer and client URLs. Nodes are expected to come back on the same IP address if rebooted.

Each member node should be running (masterless) Kubelet with a `--pod-manifest-path` configured. Etcd-wrapper will write a static pod manifest for etcd to this path to start and update the etcd instance on the node.

There is no cleanup of etcd data on recovery steps which require removing old data. Data is intended to live in the etcd container and be discarded on pod restart.

#### Workflow

- If no cluster exists, recover or start new etcd cluster.
  - Check backup location (S3), and attempt to recover.
  - If backup exists, recover existing cluster.
  - If backup can't be accessed, start new cluster.
  
- If a cluster exists but a member missing, start a new member to join the existing cluster.
  - Start a new etcd instance as an existing member.
  - If a conflicting old member is found in the cluster, remove it using etcd API.
  - Add missing member using etcd API.

- Periodic snapshot and backup to S3.

#### Sample etcd-wrapper deployed as a static pod

Nodes:
- 192.168.126.219
- 192.168.126.220
- 192.168.126.221

```
kind: Pod
apiVersion: v1
metadata:
namespace: kube-system
name: kube-etcd
spec:
hostNetwork: true
restartPolicy: Always
containers:
- name: kube-etcd-wrapper
    image: randomcoww/etcd-wrapper:20181227.02
    args:
    - "--name=$(NODE_NAME)"
    - "--cert-file=/var/lib/kubelet/etcd.pem"
    - "--key-file=/var/lib/kubelet/etcd-key.pem"
    - "--peer-cert-file=/var/lib/kubelet/etcd.pem"
    - "--peer-key-file=/var/lib/kubelet/etcd-key.pem"
    - "--trusted-ca-file=/var/lib/kubelet/ca.pem"
    - "--peer-trusted-ca-file=/var/lib/kubelet/ca.pem"
    - "--initial-advertise-peer-urls=https://$(INTERNAL_IP):52380"
    - "--listen-peer-urls=https://$(INTERNAL_IP):52380"
    - "--listen-client-urls=https://$(INTERNAL_IP):52379,https://127.0.0.1:52379"
    - "--advertise-client-urls=https://$(INTERNAL_IP):52379"
    - "--initial-cluster=controller-0=https://192.168.126.219:52380,controller-1=https://192.168.126.220:52380,controller-2=https://192.168.126.221:52380"
    - "--initial-cluster-token=etcd-default"
    - "--etcd-servers=https://192.168.126.219:52379,https://192.168.126.220:52379,https://192.168.126.221:52379"
    - "--backup-dir=/var/lib/kubelet/etcd/backup"
    - "--backup-file=/var/lib/kubelet/etcd/backup/etcd.db"
    - "--tls-dir=/var/lib/kubelet"
    - "--image=quay.io/coreos/etcd:v3.3"
    - "--pod-spec-file=/var/lib/kubelet/manifests/podspec.json"
    - "--s3-backup-path=randomcoww-etcd-backup/test-backup"
    - "--backup-interval=30m"
    - "--healthcheck-interval=10s"
    - "--pod-update-interval=2m"
    env:
    - name: AWS_ACCESS_KEY_ID
    value: "id"
    - name: AWS_SECRET_ACCESS_KEY
    value: "key"
    - name: AWS_DEFAULT_REGION
    value: "us-west-2"
    - name: AWS_SDK_LOAD_CONFIG
    value: "1"
    - name: INTERNAL_IP
    valueFrom:
        fieldRef:
        fieldPath: status.hostIP
    - name: NODE_NAME
    valueFrom:
        fieldRef:
        fieldPath: spec.nodeName
    volumeMounts:
    - name: kubernetes-path
    mountPath: "/var/lib/kubelet"
    readOnly: false
volumes:
- name: kubernetes-path
    hostPath:
    path: "/var/lib/kubelet"
``` 
