package cluster

import (
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"go.etcd.io/etcd/clientv3"
	"strings"
)

func NewMemberSetFromConfig(c *Cluster) etcdutil.MemberSet {
	memberSet := etcdutil.MemberSet{}
	// Parse node names from initial cluster
	for _, m := range strings.Split(c.InitialCluster, ",") {
		name := strings.Split(m, "=")[0]
		if len(name) > 0 {
			memberSet[name] = &etcdutil.Member{
				Name:         name,
				SecurePeer:   true,
				SecureClient: true,
			}
		}
	}
	return memberSet
}

func NewMemberSetFromList(memberList *clientv3.MemberListResponse) etcdutil.MemberSet {
	memberSet := etcdutil.MemberSet{}
	for _, m := range memberList.Members {
		memberSet[m.Name] = &etcdutil.Member{
			Name:         m.Name,
			ID:           m.ID,
			SecurePeer:   true,
			SecureClient: true,
		}
	}
	return memberSet
}
