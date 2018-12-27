### etcd-wrapper

_"I want etcd-operator like management for an etcd cluster, but without the dependecy on Kubernetes, which in turn depends on another etcd cluster"_

This is intended for managing a static on-prem etcd cluster bootstrapped with fixed peer and client URLs. Each member node should be running (masterless) Kubelet with a `--pod-manifest-path` configured. Etcd-wrapper will write a static pod manifest for etcd to this path to start and update the etcd instance on the node.

- If no cluster exists, recover or start new etcd cluster.
  - Check backup location (S3), and attempt to recover.
  - If backup exists, recover existing cluster.
  - If backup can't be accessed, start new cluster.
  
- If a cluster exists but a member missing, start a new member to join the existing cluster.
  - Start a new etcd instance as an existing member.
  - If a conflicting old member is found in the cluster, remove it using etcd API.
  - Add missing member using etcd API.

- Periodic snapshot and backup to S3.
