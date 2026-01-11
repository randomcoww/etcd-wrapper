### Dev environment

Generate certs and manifests

```bash
terraform() {
  set -x
  podman run -it --rm --security-opt label=disable \
    -v $(pwd):$(pwd) \
    -w $(pwd) \
    --net=host \
    docker.io/hashicorp/terraform:latest "$@"
  rc=$?; set +x; return $rc
}
```

```bash
terraform -chdir=test init -upgrade && terraform -chdir=test apply
```

Go build and test

```bash
ETCD_VERSION=$(curl -s https://api.github.com/repos/etcd-io/etcd/tags | grep name | head -1 | cut -d '"' -f 4)
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

Build test container

```bash
ETCD_VERSION=$(curl -s https://api.github.com/repos/etcd-io/etcd/tags | grep name | head -1 | cut -d '"' -f 4)
podman build --build-arg=ETCD_VERSION=$ETCD_VERSION -t etcd-wrapper .
```

Run test cluster

```bash
podman play kube test/outputs/minio.yaml

podman play kube test/outputs/node0.yaml
podman play kube test/outputs/node1.yaml
podman play kube test/outputs/node2.yaml
```

Check backups

```bash
podman exec minio-mc mc ls m/etcd/integ
```