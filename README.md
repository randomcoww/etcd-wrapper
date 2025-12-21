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
podman volume create \
  --driver image \
  --opt image="gcr.io/etcd-development/etcd:v3.6.6" etcdvolume

podman run -it --rm \
  -v $(pwd):/go/src \
  -v etcdvolume:/etcd \
  -w /go/src \
  --net host \
  docker.io/golang:alpine sh
```

Build test container

```bash
podman build -t etcd-wrapper .
```

Run test cluster

```bash
podman play kube test/outputs/minio.yaml

podman play kube test/outputs/node0.yaml
podman play kube test/outputs/node1.yaml
podman play kube test/outputs/node2.yaml
```