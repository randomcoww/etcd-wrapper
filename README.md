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
tw() {
  set -x
  podman run -it --rm --security-opt label=disable \
    --entrypoint='' \
    -v $(pwd):$(pwd) \
    -w $(pwd) \
    -e AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID \
    -e AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY \
    -e AWS_ENDPOINT_URL_S3=$AWS_ENDPOINT_URL_S3 \
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