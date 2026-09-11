### Generate certs and manifests

```bash
tofu() {
  set -x
  podman run -it --rm --security-opt label=disable \
    -v $(pwd):$(pwd) \
    -w $(pwd) \
    --net=host \
    ghcr.io/opentofu/opentofu:latest "$@"
  rc=$?; set +x; return $rc
}
```

```bash
tofu -chdir=test init -upgrade && tofu -chdir=test apply
```

Add etcd binary for testing and run go env

```bash
ETCD_VERSION=$(curl -s https://api.github.com/repos/etcd-io/etcd/releases/latest | grep tag_name | cut -d '"' -f 4 | tr -d 'v')
podman volume rm etcdvolume -f
podman pull registry.k8s.io/etcd:$ETCD_VERSION
podman volume create \
  --driver image \
  --opt image="registry.k8s.io/etcd:$ETCD_VERSION" etcdvolume

podman run -it --rm \
  -v $(pwd):/go/src \
  -v etcdvolume:/etcd \
  -w /go/src \
  --net host \
  docker.io/golang:alpine sh
```

### Build test container

```bash
podman build -t etcd-wrapper .
```

### Run test cluster

```bash
podman play kube test/outputs/node0-wrapper.yaml
podman play kube test/outputs/node1-wrapper.yaml
podman play kube test/outputs/node2-wrapper.yaml
```

### Check backups

```bash
podman exec minio-mc mc ls m/etcd/integ
```

### Run etcd only

```bash
podman play kube test/outputs/node0.yaml
podman play kube test/outputs/node1.yaml
podman play kube test/outputs/node2.yaml
```

### Test query

```bash
podman exec -it node0-etcd etcdctl --endpoints=https://127.0.0.1:8080 \
  --cacert=/var/lib/etcd/client/ca.crt \
  --cert=/var/lib/etcd/client/tls.crt \
  --key=/var/lib/etcd/client/tls.key \
  member list
```