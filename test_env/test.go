package main

import (
	"fmt"
	"io/ioutil"

	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
)

func main() {
	cert, _ := ioutil.ReadFile("generate_cluster/output/etcd-0-client.pem")
	key, _ := ioutil.ReadFile("generate_cluster/output/etcd-0-client-key.pem")
	ca, _ := ioutil.ReadFile("generate_cluster/output/root-client-ca.pem")

	clientURLs := []string{
		"https://127.0.0.1:9001",
		"https://127.0.0.1:9002",
		"https://127.0.0.1:9003",
	}

	tlsConfig, err := etcdutil.NewTLSConfig(cert, key, ca)
	if err != nil {
		fmt.Errorf("TLS config failed: %v", err)
	}

	memberList, err := etcdutil.ListMembers(clientURLs, tlsConfig)
	if err != nil {
		fmt.Errorf("List members failed: %v", err)
	}
	fmt.Printf("List members: %+v\n", memberList)

	status, err := etcdutilextra.Status(clientURLs, tlsConfig)
	if err != nil {
		fmt.Errorf("Status failed: %v", err)
	}
	fmt.Printf("Status: %+v\n", status)
}
