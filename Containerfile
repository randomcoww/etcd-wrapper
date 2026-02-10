FROM docker.io/golang:alpine as build

WORKDIR /go/src
COPY . .

RUN set -x \
  \
  && apk add --no-cache \
    git \
  \
  && CGO_ENABLED=0 GO111MODULE=on GOOS=linux go build -v -ldflags '-s -w' -o etcd-wrapper main.go

# TODO: Remove baking in etcd binaries once ImageVolumes work
ARG ETCD_VERSION
FROM registry.k8s.io/etcd:$ETCD_VERSION as etcd

COPY --from=build /go/src/etcd-wrapper /usr/local/bin/