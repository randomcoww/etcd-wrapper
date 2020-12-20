module github.com/randomcoww/etcd-wrapper

go 1.15

replace github.com/coreos/bbolt => go.etcd.io/bbolt v1.3.5

replace google.golang.org/grpc => google.golang.org/grpc v1.26.0

require (
	github.com/aws/aws-sdk-go v1.36.12
	github.com/coreos/bbolt v0.0.0-00010101000000-000000000000 // indirect
	github.com/coreos/etcd v3.3.25+incompatible // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/prometheus/client_golang v1.9.0 // indirect
	github.com/sirupsen/logrus v1.7.0
	go.etcd.io/etcd v3.3.25+incompatible
	go.uber.org/zap v1.16.0 // indirect
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
)
