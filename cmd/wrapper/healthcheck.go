package wrapper

import (
	"fmt"
	"strings"
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
					if clusterErrCount >= (h.config.ClusterErrThreshold * 3) {
						// Trigger pod restart if this fails too many times
						logrus.Infof("Cluster healthcheck failed too many times (%v): %v", clusterErrCount, err)
						h.config.UpdateInstance()
						clusterErrCount = 0
					} else {
						logrus.Errorf("Cluster healthcheck failed: %v", err)
					}

					h.config.SendMissingNew()
				}
				continue
			}
			clusterErrCount = 0

			// logrus.Infof("MemberList: %#v", memberList.Members)
			// Populate all members
			// Populate local member ID or 0 if not found
			h.mergeMemberSet(memberList)

			err = etcdutilextra.HealthCheck(h.config.LocalClientURLs, h.config.TLSConfig)
			if err != nil {
				localErrCount++

				if localErrCount >= h.config.LocalErrThreshold {
					if localErrCount >= (h.config.LocalErrThreshold * 3) {
						// Trigger pod restart if this fails too many times
						logrus.Infof("Local healthcheck failed too many times (%v): %v", localErrCount, err)
						h.config.UpdateInstance()
						localErrCount = 0
					} else {
						logrus.Infof("Local healthcheck failed (%v): %v", localErrCount, err)
					}

					h.config.SendMissingExisting()

					if h.localID != 0 {
						if err := h.removeMember(h.localID); err != nil {
							continue
						}
						if _, err := h.addMember(); err != nil {
							continue
						}
						localErrCount = 0
					} else {
						if _, err := h.addMember(); err != nil {
							continue
						}
						localErrCount = 0
					}
				}
				continue
			}
			localErrCount = 0

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
	var localID uint64
	var log []string

	for _, m := range memberList.Members {
		log = append(log, fmt.Sprintf("%v (%v)", m.ID, m.Name))

		// My ID matched by peerURL
		// It may not have a name yet if it was recently added
		if config.IsEqual(m.PeerURLs, h.config.LocalPeerURLs) {
			localID = m.ID
		}

		// New members may not have names yet
		if len(m.Name) == 0 {
			continue
		}

		if member, ok := h.memberSet[m.Name]; ok {
			// logrus.Infof("Found member: %v (%v)", m.Name, m.ID)
			member.id = m.ID
			memberFoundList[m.Name] = member
		} else {
			logrus.Errorf("Unknown member: %v (%v)", m.Name, m.ID)
			h.removeMember(m.ID)
		}
	}
	logrus.Infof("Found members: %s", strings.Join(log, ", "))

	h.localID = localID
	logrus.Infof("Local ID: %v", h.localID)

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
