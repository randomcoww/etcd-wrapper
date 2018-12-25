package wrapper

import (
	"time"

	"go.etcd.io/etcd/clientv3"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
)

type Member struct {
	id uint64
}

type MemberSet map[string]*Member

type HealthCheck struct {
	config           *config.Config
	memberSet        MemberSet
	localID          uint64
	localErrCount    int
	clusterErrCount  int
	localCheckStop   chan struct{}
	clusterCheckStop chan struct{}
}

func newHealthCheck(c *config.Config) *HealthCheck {
	h := &HealthCheck{
		config:           c,
		memberSet:        MemberSet{},
		localCheckStop:   make(chan struct{}, 1),
		clusterCheckStop: make(chan struct{}, 1),
	}

	for _, memberName := range h.config.MemberNames {
		h.memberSet[memberName] = &Member{}
	}

	return h
}

func (h *HealthCheck) runLocalCheck() {
	logrus.Infof("Start healthcheck handler")

	for {
		select {
		case <-time.After(h.config.HealthCheckInterval):
			err := etcdutilextra.HealthCheck(h.config.LocalClientURLs, h.config.TLSConfig)
			if err != nil {
				h.localErrCount += 1
				logrus.Errorf("Healthcheck failed (%v): %v", h.localErrCount, err)

				if h.localErrCount >= h.config.LocalErrThreshold {
					// Remove local member
					logrus.Errorf("Healthcheck failed too many times")
					h.localErrCount = 0
					if h.localID != 0 {
						h.config.SendLocalRemove(h.localID)
					}
				}
				continue
			}
			h.localErrCount = 0
			logrus.Infof("Healthcheck success")

		case <-h.localCheckStop:
			return
		}
	}
}

func (h *HealthCheck) runClusterCheck() {
	logrus.Infof("Start cluster healthcheck handler")

	for {
		select {
		case <-time.After(h.config.HealthCheckInterval):
			memberList, err := etcdutil.ListMembers(h.config.ClientURLs, h.config.TLSConfig)
			if err != nil {
				// Update annotation to restart pod
				h.clusterErrCount += 1
				logrus.Errorf("List members failed (%v): %v", h.clusterErrCount, err)

				if h.clusterErrCount >= h.config.ClusterErrThreshold {
					logrus.Errorf("List members failed too many times")
					// Recover backup or create new cluster
					h.clusterErrCount = 0
					h.config.SendMissingNew()
				}
				continue
			}
			h.clusterErrCount = 0

			// Populate all members
			h.mergeMemberSet(memberList)

			// Populate ID of my node
			if _, ok := h.memberSet[h.config.Name]; ok {
				h.localID = h.memberSet[h.config.Name].id
			}

			// My local node is missing
			if h.localID == 0 {
				// Create member as existing
				logrus.Errorf("Local member not found: %v", h.config.Name)
				h.config.SendMissingExisting()
			}
		case <-h.clusterCheckStop:
			return
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

func (h *HealthCheck) stopRun() {
	select {
	case h.clusterCheckStop <- struct{}{}:
	default:
	}

	select {
	case h.localCheckStop <- struct{}{}:
	default:
	}
}
