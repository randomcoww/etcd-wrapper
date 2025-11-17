### Dev environment

```bash
podman run -it --rm \
  -v $(pwd):/go/src \
  -w /go/src \
  docker.io/golang:alpine sh
```

#### Local etcd cluster

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
terraform -chdir=test init -upgrade && \
terraform -chdir=test apply
```