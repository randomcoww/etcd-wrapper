### Dev

```
podman run -it --rm \
  -v $(pwd):/go/src/etcd-wrapper \
  -w /go/src/etcd-wrapper \
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

Run wrapper instances

```bash
podman play kube testenv/output/node0.yaml & \
podman play kube testenv/output/node1.yaml & \
podman play kube testenv/output/node2.yaml
```

Monitor logs

```bash
podman logs -f etcd-node0-wrapper-etcd-wrapper
podman logs -f etcd-node1-wrapper-etcd-wrapper
podman logs -f etcd-node2-wrapper-etcd-wrapper
```

Cleanup

```bash
podman play kube testenv/output/node0.yaml --down & \
podman play kube testenv/output/node1.yaml --down & \
podman play kube testenv/output/node2.yaml --down

tw terraform -chdir=testenv destroy
```

Cleanup formatting

```bash
tw find . -name '*.tf' -exec terraform fmt '{}' \;
```