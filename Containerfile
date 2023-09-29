
FROM golang:alpine as BUILD

WORKDIR /go/src/github.com/randomcoww/etcd-wrapper
COPY . .

RUN set -x \
  \
  && apk add --no-cache \
    git \
  \
  && CGO_ENABLED=0 GO111MODULE=on GOOS=linux go build -v -ldflags '-s -w' -o etcd-wrapper main.go

FROM alpine:latest

COPY --from=BUILD /go/src/github.com/randomcoww/etcd-wrapper/etcd-wrapper /

RUN set -x \
  \
  && apk add --no-cache \
    ca-certificates \
  && update-ca-certificates
 
ENTRYPOINT ["/etcd-wrapper"]
