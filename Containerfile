# TODO: Remove baking in etcd binaries once ImageVolumes work
ARG ETCD_VERSION
FROM registry.k8s.io/etcd:$ETCD_VERSION as etcd

FROM docker.io/golang:alpine as build

WORKDIR /go/src
COPY . .

RUN set -x \
  \
  && apk add --no-cache \
    git \
    ca-certificates \
  \
  && update-ca-certificates \
  && CGO_ENABLED=0 GO111MODULE=on GOOS=linux go build -v -ldflags '-s -w' -o etcd-wrapper main.go

FROM scratch

COPY --from=etcd /usr/local/bin/etcd /bin/
COPY --from=etcd /usr/local/bin/etcdutl /bin/
COPY --from=build /go/src/etcd-wrapper /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["/bin/etcd-wrapper"]
