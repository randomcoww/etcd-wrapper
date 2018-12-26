package wrapper

import (
	"time"

	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
)

type Member struct {
	id uint64
}

type MemberSet map[string]*Member

type HealthCheck struct {
	config    *config.Config
	memberSet MemberSet
	localID   uint64
	stop      chan struct{}
}

func newHealthCheck(c *config.Config) *HealthCheck {
	h := &HealthCheck{
		config:    c,
		memberSet: MemberSet{},
		stop:      make(chan struct{}, 1),
	}

	for _, memberName := range h.config.MemberNames {
		h.memberSet[memberName] = &Member{}
	}
	return h
}

func (h *HealthCheck) runPeriodic() {
	logrus.Infof("Start periodic healthcheck handler")

	clusterErrCount := 0
	localErrCount := 0

	for {
		select {
		case <-time.After(h.config.HealthCheckInterval):
			memberList, err := etcdutil.ListMembers(h.config.ClientURLs, h.config.TLSConfig)
			if err != nil {
				clusterErrCount++
				if clusterErrCount >= h.config.ClusterErrThreshold {
					logrus.Errorf("Cluster healthcheck failed: %v", err)
					h.config.SendMissingNew()
				}
				continue
			}
			clusterErrCount = 0

			// Populate all members
			h.mergeMemberSet(memberList)

			err = etcdutilextra.HealthCheck(h.config.LocalClientURLs, h.config.TLSConfig)
			if err != nil {
				localErrCount++

				if localErrCount >= h.config.LocalErrThreshold {
					logrus.Infof("Local healthcheck failed: %v", err)
					h.config.SendMissingExisting()

					if h.localID != 0 {
						if err := h.removeMember(h.localID); err == nil {
							h.localID = 0
						} else {
							continue
						}
					}

					if resp, err := h.addMember(); err == nil {
						h.localID = resp.Member.ID
					}
				}
				continue
			}
			localErrCount = 0

			// Populate ID of my node
			if _, ok := h.memberSet[h.config.Name]; ok {
				h.localID = h.memberSet[h.config.Name].id
				logrus.Infof("Found local member ID: %v", h.localID)
			}

		case <-h.stop:
			return
		}
	}
}

func (h *HealthCheck) stopRun() {
	select {
	case h.stop <- struct{}{}:
	default:
	}
}

func (h *HealthCheck) mergeMemberSet(memberList *clientv3.MemberListResponse) MemberSet {
	memberFoundList := MemberSet{}
	for _, m := range memberList.Members {
		// New members may not have names yet
		if len(m.Name) == 0 {
			continue
		}

		if member, ok := h.memberSet[m.Name]; ok {
			logrus.Infof("Found member: %v (%v)", m.Name, m.ID)
			member.id = m.ID
			memberFoundList[m.Name] = member
		} else {
			logrus.Errorf("Unknown member: %v (%v)", m.Name, m.ID)
			h.removeMember(m.ID)
		}
	}

	// Go through members in config not returned by etcd and reset ID
	for memberName, member := range h.memberSet {
		if _, ok := memberFoundList[memberName]; !ok {
			member.id = 0
		}
	}
	return memberFoundList
}

func (h *HealthCheck) addMember() (*clientv3.MemberAddResponse, error) {
	resp, err := etcdutilextra.AddMember(h.config.ClientURLs, h.config.LocalPeerURLs, h.config.TLSConfig)
	switch err {
	case nil:
		logrus.Infof("Add member success: %v", resp.Member.ID)
		return resp, nil
	default:
		logrus.Errorf("Add member failed: %v", err)
		return nil, err
	}
}

func (h *HealthCheck) removeMember(memberID uint64) error {
	err := etcdutil.RemoveMember(h.config.ClientURLs, h.config.TLSConfig, memberID)
	switch err {
	case nil, rpctypes.ErrMemberNotFound:
		logrus.Infof("Remove member success: %v", memberID)
		return nil
	default:
		logrus.Errorf("Remove member failed (%v): %v", memberID, err)
		return err
	}
}
