FROM docker.io/golang:alpine@sha256:b6ed3fd0452c0e9bcdef5597f29cc1418f61672e9d3a2f55bf02e7222c014abd as build

WORKDIR /go/src
COPY . .

RUN set -x \
  \
  && apk add --no-cache \
    git \
    ca-certificates \
  && update-ca-certificates \
  \
  && CGO_ENABLED=0 GO111MODULE=on GOOS=linux \
    go build -v -ldflags '-s -w' -o etcd-wrapper cmd/main.go \
  && go test ./...

FROM scratch

COPY --from=build /go/src/etcd-wrapper /bin/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
 
ENTRYPOINT ["/bin/etcd-wrapper"]
