module github.com/randomcoww/etcd-wrapper

go 1.13

require (
	cloud.google.com/go/storage v1.4.0 // indirect
	github.com/Azure/azure-sdk-for-go v36.2.0+incompatible // indirect
	github.com/Azure/go-autorest v11.1.2+incompatible
	github.com/aws/aws-sdk-go v1.25.43
	github.com/coreos/etcd v3.3.18+incompatible // indirect
	github.com/coreos/etcd-operator v0.9.4
	github.com/coreos/go-systemd v0.0.0-00010101000000-000000000000 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/sirupsen/logrus v1.4.2
	go.etcd.io/etcd v3.3.18+incompatible
	go.uber.org/zap v1.13.0 // indirect
	google.golang.org/grpc v1.25.1 // indirect
	k8s.io/api v0.0.0-20191121015604-11707872ac1c
	k8s.io/apimachinery v0.0.0-20191123233150-4c4803ed55e3
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab // indirect
	k8s.io/utils v0.0.0-20191114200735-6ca3b61696b6 // indirect
)

replace github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.0.0

replace github.com/coreos/etcd-operator => ./vendor/github.com/coreos/etcd-operator
