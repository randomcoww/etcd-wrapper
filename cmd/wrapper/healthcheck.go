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
	cs := &HealthCheck{
		config:           c,
		members:          make(map[string]*Member),
		localCheckStop:   make(chan struct{}, 1),
		clusterCheckStop: make(chan struct{}, 1),
	}

	// Populate members from config
	for _, memberName := range cs.config.MemberNames {
		cs.members[memberName] = &Member{}
	}
	return cs
}

func (h *HealthCheck) runLocalCheck() {
	logrus.Infof("Start healthcheck handler")

	for {
		select {
		case <-time.After(h.config.HealthCheckInterval):
			err := etcdutilextra.HealthCheck(h.config.LocalClientURLs, h.config.TLSConfig)
			if err != nil {
				logrus.Errorf("Healthcheck failed (%v): %v", h.localErrCount, err)
				h.localErrCount += 1

				if h.localErrCount >= h.config.LocalErrThreshold {
					// Remove local member
					if h.localID != 0 {
						select {
						case h.config.NotifyLocalRemove <- h.localID:
						default:
						}
					}
				}
				continue
			}
			h.localErrCount = 0

		case <-h.localCheckStop:
			return
		}
	}
}

func (h *HealthCheck) runClusterCheck() {
	logrus.Infof("Start cluster state handler")

	for {
		select {
		case <-time.After(h.config.HealthCheckInterval):
			memberList, err := etcdutil.ListMembers(h.config.ClientURLs, h.config.TLSConfig)
			if err != nil {
				// Update annotation to restart pod
				logrus.Errorf("Could not get member list (%v): %v", h.clusterErrCount, err)
				h.clusterErrCount += 1

				if h.clusterErrCount >= h.config.ClusterErrThreshold {
					logrus.Errorf("List member failed too many times")
					// Recover backup or create new cluster
					select {
					case h.config.NotifyMissingNew <- struct{}{}:
					default:
					}
				}
				continue
			}
			h.clusterErrCount = 0

			// Populate all members
			for _, member := range memberList.Members {
				if _, ok := h.members[member.Name]; ok {
					logrus.Infof("Found member: %v (%v)", member.Name, member.ID)
					h.members[member.Name].id = member.ID
				} else {
					// Removed unknown member ID
					select {
					case h.config.NotifyRemoteRemove <- member.ID:
					default:
					}
				}
			}

			// Populate ID of my node
			if _, ok := h.members[h.config.Name]; ok {
				h.localID = h.members[h.config.Name].id
			}

			// My local node is missing
			if h.localID == 0 {
				// Create member as existing
				select {
				case h.config.NotifyMissingExisting <- struct{}{}:
				default:
				}
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
