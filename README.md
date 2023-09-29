### Image build

```
TAG=ghcr.io/randomcoww/etcd-wrapper:$(date -u +'%Y%m%d')

podman build \
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
    -v $HOME/.aws:/root/.aws \
    -w $(pwd) \
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
podman play kube testenv/output/node0.yaml
podman play kube testenv/output/node1.yaml
podman play kube testenv/output/node2.yaml
```

Start etcd

```bash
podman play kube testenv/output/node0/manifests/etcd.json
podman play kube testenv/output/node1/manifests/etcd.json
podman play kube testenv/output/node2/manifests/etcd.json
```

Cleanup formatting

```bash
tw find . -name '*.tf' -exec terraform fmt '{}' \;
```