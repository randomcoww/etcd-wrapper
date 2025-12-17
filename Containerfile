FROM docker.io/golang:alpine as build

WORKDIR /go/src
COPY . .

RUN set -x \
  \
  && apk add --no-cache \
    git \
  \
  && CGO_ENABLED=0 GO111MODULE=on GOOS=linux \
    go build -v -ldflags '-s -w' -o etcd-wrapper main.go

FROM scratch

COPY --from=build /go/src/etcd-wrapper /bin/

ENTRYPOINT ["/bin/etcd-wrapper"]
