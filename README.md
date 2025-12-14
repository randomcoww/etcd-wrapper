### Dev environment

```bash
podman volume create \
  --driver image \
  --opt image="gcr.io/etcd-development/etcd:v3.6.6" etcdvolume

podman run -it --rm \
  -v $(pwd):/go/src \
  -v etcdvolume:/mnt \
  -w /go/src \
  --net host \
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
terraform -chdir=tf init -upgrade && \
terraform -chdir=tf apply
```