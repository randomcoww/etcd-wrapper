package wrapper

import (
	"time"

	"go.etcd.io/etcd/clientv3"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"

	// "github.com/randomcoww/etcd-wrapper/pkg/backup"
	// "go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
)

type Member struct {
	id uint64
}

type MemberSet map[string]*Member

type HealthCheck struct {
	config           *config.Config
	memberSet        MemberSet
	localID          uint64
	// clusterErrCh chan struct{}
	// localErrCh chan struct{}
}

func newHealthCheck(c *config.Config) *HealthCheck {
	h := &HealthCheck{
		config:           c,
		memberSet:        MemberSet{},
		// clusterErrCh make(chan struct{}, 1),
		// localErrCh make(chan struct{}, 1),
	}

	for _, memberName := range h.config.MemberNames {
		h.memberSet[memberName] = &Member{}
	}

	return h
}

func (h *HealthCheck) runClusterCheck() {
	clusterErrCount := 0
	clusterErrThreshold := 3
	localErrCount := 0
	localErrThreshold := 3

	for {
		select {
		case <-time.After(20 * time.Second):
			// Check cluster
			memberList, err := etcdutil.ListMembers(h.config.ClientURLs, h.config.TLSConfig)
			if err != nil {
				clusterErrCount++
				// h.clusterErr = true

				if clusterErrCount >= clusterErrThreshold {
					// select {
					// case h.clusterErrCh <- struct{}{}:
					// default:
					// }
					h.config.SendMissingNew()
				}
				continue
			}
			clusterErrCount = 0
			// h.clusterErr = false

			// Populate all members
			h.mergeMemberSet(memberList)

			// Populate ID of my node
			if _, ok := h.memberSet[h.config.Name]; ok {
				h.localID = h.memberSet[h.config.Name].id
			}

			// cluster err overrides this check
			err = etcdutilextra.HealthCheck(h.config.LocalClientURLs, h.config.TLSConfig)
			if err != nil {
				localErrCount++

				if localErrCount >= localErrThreshold {
					// select {
					// case h.localErrCh <- struct{}{}:
					// default:
					// }
					h.config.SendMissingExisting()

					if h.localID != 0 {
						// removeMember(h.config, h.localID)
						h.config.SendLocalRemove(h.localID)
						h.config.SendLocalAdd()
					} else {
						// addMember(h.config)
						h.config.SendLocalAdd()
					}
				}
				continue
			}
			localErrCount = 0
		}
	}
}

func (h *HealthCheck) mergeMemberSet(memberList *clientv3.MemberListResponse) {
	membersFound := MemberSet{}
	for _, m := range memberList.Members {
		// New members may not have names yet
		if len(m.Name) == 0 {
			continue
		}

		if member, ok := h.memberSet[m.Name]; ok {
			logrus.Infof("Found member: %v (%v)", m.Name, m.ID)
			member.id = m.ID
			membersFound[m.Name] = member
		} else {
			logrus.Errorf("Unknown member: %v (%v)", m.Name, m.ID)
			h.config.SendRemoteRemove(m.ID)
		}
	}

	// Go through members in config not returned by etcd and reset ID
	for memberName, member := range h.memberSet {
		if _, ok := membersFound[memberName]; !ok {
			member.id = 0
		}
	}
}
