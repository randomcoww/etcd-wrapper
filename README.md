### Dev

```
podman run -it --rm \
  -v $(pwd):/go/src \
  -w /go/src \
  docker.io/golang:alpine sh
```