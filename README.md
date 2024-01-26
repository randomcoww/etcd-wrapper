### Image build

```
TAG=ghcr.io/randomcoww/etcd-wrapper:$(date -u +'%Y%m%d').5

mkdir -p build
TMPDIR=$(pwd)/build podman build \
  -t $TAG . && \

podman push $TAG
```

### Dev build

```
podman run -it --rm \
  -v $(pwd):/go/etcd-wrapper \
  -w /go/etcd-wrapper \
   golang:alpine sh
```

### Test environment

Define the `tw` (terraform wrapper) command

```bash
mkdir -p $HOME/.aws

tw() {
  set -x
  podman run -it --rm --security-opt label=disable \
    --entrypoint='' \
    -v $(pwd):$(pwd) \
    -w $(pwd) \
    -v $HOME/.aws:/root/.aws \
    --net=host \
    docker.io/hashicorp/terraform:latest "$@"
  rc=$?; set +x; return $rc
}
```

```bash
tw terraform -chdir=testenv init
tw terraform -chdir=testenv apply
```

Run each node

```bash
podman play kube testenv/output/node0.yaml & \
podman play kube testenv/output/node1.yaml & \
podman play kube testenv/output/node2.yaml

podman play kube testenv/output/node0.yaml --down & \
podman play kube testenv/output/node1.yaml --down & \
podman play kube testenv/output/node2.yaml --down
```

```bash
podman logs -f etcd-wrapper-node0-etcd-wrapper
podman logs -f etcd-wrapper-node1-etcd-wrapper
podman logs -f etcd-wrapper-node2-etcd-wrapper
```

```bash
podman logs -f etcd-node0-etcd
podman logs -f etcd-node1-etcd
podman logs -f etcd-node2-etcd
```

Cleanup formatting

```bash
tw find . -name '*.tf' -exec terraform fmt '{}' \;
```