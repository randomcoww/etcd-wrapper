
FROM golang:alpine as BUILD

WORKDIR /go/src/github.com/randomcoww/etcd-wrapper
COPY . .

RUN set -x \
  \
  && apk add --no-cache \
    git \
  \
  && GO111MODULE=on GOOS=linux \
    go build -o etcd-wrapper main.go

FROM alpine:edge

COPY --from=BUILD /go/src/github.com/randomcoww/etcd-wrapper/etcd-wrapper /

RUN set -x \
  \
  && apk add --no-cache \
    ca-certificates
 
ENTRYPOINT ["/etcd-wrapper"]
