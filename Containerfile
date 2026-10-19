FROM docker.io/golang:alpine as build

WORKDIR /go/src
COPY . .

RUN set -x \
  \
  && apk add --no-cache \
    git \
    ca-certificates \
  \
  && CGO_ENABLED=0 GO111MODULE=on GOOS=linux go build -v -ldflags '-s -w' cmd/etcdwrapper.go

FROM scratch

ENV SSL_CERT_FILE /etc/ssl/certs/ca-certificates.crt

COPY --from=build /go/src/etcdwrapper /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt