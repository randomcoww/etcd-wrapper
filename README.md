### Dev

```
podman run -it --rm \
  -v $(pwd):/go/src/etcd-wrapper \
  -w /go/src/etcd-wrapper \
   docker.io/golang:alpine sh
```

### Test environment

Run Terraform in container (optional)

```bash
terraform() {
  set -x
  podman run -it --rm --security-opt label=disable \
    -v $(pwd):$(pwd) \
    -w $(pwd) \
    --env-file=credentials.env \
    --net=host \
    docker.io/hashicorp/terraform:latest "$@"
  rc=$?; set +x; return $rc
}
```

```bash
terraform -chdir=testenv init
terraform -chdir=testenv apply
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

terraform -chdir=testenv destroy
```