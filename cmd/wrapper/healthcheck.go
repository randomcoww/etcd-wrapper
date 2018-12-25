package wrapper

import (
	"time"

	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	etcdutilextra "github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
)

type Member struct {
	id uint64
}

type HealthCheck struct {
	config           *config.Config
	members          map[string]*Member
	localID          uint64
	localErrCount    int
	clusterErrCount  int
	localCheckStop   chan struct{}
	clusterCheckStop chan struct{}
}

func newHealthCheck(c *config.Config) *HealthCheck {
	h := &HealthCheck{
		config:           c,
		members:          make(map[string]*Member),
		localCheckStop:   make(chan struct{}, 1),
		clusterCheckStop: make(chan struct{}, 1),
	}

	// Populate members from config
	for _, memberName := range h.config.MemberNames {
		h.members[memberName] = &Member{}
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
			for _, member := range memberList.Members {
				// New members can have blank name
				if len(member.Name) == 0 {
					continue
				}

				if _, ok := h.members[member.Name]; ok {
					// logrus.Infof("Found member: %v (%v)", member.Name, member.ID)
					h.members[member.Name].id = member.ID
				} else {
					// Removed unknown member ID
					logrus.Errorf("Found unknown member: %v (%v)", member.Name, member.ID)
					h.config.SendRemoteRemove(member.ID)
				}
			}
			logrus.Infof("List members success: %s", h.members)

			// Populate ID of my node
			if _, ok := h.members[h.config.Name]; ok {
				h.localID = h.members[h.config.Name].id
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
