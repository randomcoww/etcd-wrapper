package status

import (
	"crypto/tls"
	"errors"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func uint64ptr(i uint64) *uint64 {
	return &i
}

func int64ptr(i int64) *int64 {
	return &i
}

type mockStatusResponse struct {
	delay time.Duration
	resp  *etcdutil.StatusResp
	err   error
}

type mockEtcdutil struct {
	statusResponse        error
	statusHandlerResponse []*mockStatusResponse
	listMembersResponse   []*etcdutil.MemberResp
	listMembersError      error
	listMembersEndpoints  []string
	healthCheckResponse   error
	healthCheckEndpoints  []string
}

func (v *mockEtcdutil) Status(endpoints []string, handler func(*etcdutil.StatusResp, error), tlsConfig *tls.Config) error {
	if v.statusResponse != nil {
		return v.statusResponse
	}
	var wg sync.WaitGroup
	for _, status := range v.statusHandlerResponse {
		wg.Add(1)
		go func(status *mockStatusResponse) {
			time.Sleep(status.delay)
			handler(status.resp, status.err)
			wg.Done()
		}(status)
	}
	wg.Wait()
	return nil
}

func (v *mockEtcdutil) ListMembers(endpoints []string, tlsConfig *tls.Config) ([]*etcdutil.MemberResp, error) {
	v.healthCheckEndpoints = endpoints
	return v.listMembersResponse, v.listMembersError
}

func (v *mockEtcdutil) HealthCheck(endpoints []string, tlsConfig *tls.Config) error {
	v.listMembersEndpoints = endpoints
	return v.healthCheckResponse
}

func TestNewStatus(t *testing.T) {
	args := &arg.Args{
		Name: "node0",
		InitialCluster: []*arg.Node{
			&arg.Node{
				Name:      "node0",
				PeerURL:   "https://10.0.0.1:8080",
				ClientURL: "https://10.0.0.1:8081",
			},
			&arg.Node{
				Name:      "node1",
				PeerURL:   "https://10.0.0.2:8080",
				ClientURL: "https://10.0.0.2:8081",
			},
			&arg.Node{
				Name:      "node2",
				PeerURL:   "https://10.0.0.3:8080",
				ClientURL: "https://10.0.0.3:8081",
			},
		},
	}

	tests := []struct {
		label                   string
		args                    *arg.Args
		etcdutilResponses       *mockEtcdutil
		expectedHealthy         bool
		expectedClusterID       *uint64
		expectedLeaderID        *uint64
		expectedRevision        *int64
		expectedHealthEndpoints []string
		expectedFinalEndpoints  []string
		expectedMemberMap       map[string]*Member
	}{
		{
			label: "happy path",
			args:  args,
			etcdutilResponses: &mockEtcdutil{
				statusResponse:       nil,
				listMembersError:     nil,
				healthCheckResponse:  nil,
				listMembersEndpoints: []string{},
				healthCheckEndpoints: []string{},
				statusHandlerResponse: []*mockStatusResponse{
					&mockStatusResponse{
						delay: 100 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.1:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(1),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(99),
						},
						err: nil,
					},
					&mockStatusResponse{
						delay: 300 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.2:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(2),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(101),
						},
						err: nil,
					},
					&mockStatusResponse{
						delay: 200 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.3:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(3),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(100),
						},
						err: nil,
					},
				},
				listMembersResponse: []*etcdutil.MemberResp{
					&etcdutil.MemberResp{
						ID: uint64ptr(1),
						PeerURLs: []string{
							"https://10.0.0.1:8080",
						},
					},
					&etcdutil.MemberResp{
						ID: uint64ptr(2),
						PeerURLs: []string{
							"https://10.0.0.2:8080",
						},
					},
					&etcdutil.MemberResp{
						ID: uint64ptr(3),
						PeerURLs: []string{
							"https://10.0.0.3:8080",
						},
					},
				},
			},
			expectedHealthy:         true,
			expectedClusterID:       uint64ptr(10),
			expectedLeaderID:        uint64ptr(1),
			expectedRevision:        int64ptr(101),
			expectedHealthEndpoints: []string{"https://10.0.0.1:8081", "https://10.0.0.2:8081", "https://10.0.0.3:8081"},
			expectedFinalEndpoints:  []string{"https://10.0.0.1:8081", "https://10.0.0.2:8081", "https://10.0.0.3:8081"},
			expectedMemberMap: map[string]*Member{
				"node0": &Member{
					Name:                "node0",
					Healthy:             true,
					MemberID:            uint64ptr(1),
					MemberIDFromCluster: uint64ptr(1),
					Revision:            int64ptr(99),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.1:8080",
					ClientURL:           "https://10.0.0.1:8081",
					Self:                true,
				},
				"node1": &Member{
					Name:                "node1",
					Healthy:             true,
					MemberID:            uint64ptr(2),
					MemberIDFromCluster: uint64ptr(2),
					Revision:            int64ptr(101),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.2:8080",
					ClientURL:           "https://10.0.0.2:8081",
					Self:                false,
				},
				"node2": &Member{
					Name:                "node2",
					Healthy:             true,
					MemberID:            uint64ptr(3),
					MemberIDFromCluster: uint64ptr(3),
					Revision:            int64ptr(100),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.3:8080",
					ClientURL:           "https://10.0.0.3:8081",
					Self:                false,
				},
			},
		},
		{
			label: "node0 in another cluster ID",
			args:  args,
			etcdutilResponses: &mockEtcdutil{
				statusResponse:       nil,
				listMembersError:     nil,
				healthCheckResponse:  nil,
				listMembersEndpoints: []string{},
				healthCheckEndpoints: []string{},
				statusHandlerResponse: []*mockStatusResponse{
					&mockStatusResponse{
						delay: 100 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.1:8081",
							ClusterID: uint64ptr(20),
							MemberID:  uint64ptr(6),
							LeaderID:  uint64ptr(8),
							Revision:  int64ptr(400),
						},
						err: nil,
					},
					&mockStatusResponse{
						delay: 300 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.2:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(2),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(101),
						},
						err: nil,
					},
					&mockStatusResponse{
						delay: 200 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.3:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(3),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(100),
						},
						err: nil,
					},
				},
				listMembersResponse: []*etcdutil.MemberResp{
					&etcdutil.MemberResp{
						ID: uint64ptr(2),
						PeerURLs: []string{
							"https://10.0.0.2:8080",
						},
					},
					&etcdutil.MemberResp{
						ID: uint64ptr(3),
						PeerURLs: []string{
							"https://10.0.0.3:8080",
						},
					},
				},
			},
			expectedHealthy:         true,
			expectedClusterID:       uint64ptr(10),
			expectedLeaderID:        uint64ptr(1),
			expectedRevision:        int64ptr(101),
			expectedHealthEndpoints: []string{"https://10.0.0.2:8081", "https://10.0.0.3:8081"},
			expectedFinalEndpoints:  []string{"https://10.0.0.2:8081", "https://10.0.0.3:8081"},
			expectedMemberMap: map[string]*Member{
				"node0": &Member{
					Name:                "node0",
					Healthy:             false,
					MemberID:            uint64ptr(6),
					MemberIDFromCluster: nil,
					Revision:            int64ptr(400),
					ClusterID:           uint64ptr(20),
					LeaderID:            uint64ptr(8),
					PeerURL:             "https://10.0.0.1:8080",
					ClientURL:           "https://10.0.0.1:8081",
					Self:                true,
				},
				"node1": &Member{
					Name:                "node1",
					Healthy:             true,
					MemberID:            uint64ptr(2),
					MemberIDFromCluster: uint64ptr(2),
					Revision:            int64ptr(101),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.2:8080",
					ClientURL:           "https://10.0.0.2:8081",
					Self:                false,
				},
				"node2": &Member{
					Name:                "node2",
					Healthy:             true,
					MemberID:            uint64ptr(3),
					MemberIDFromCluster: uint64ptr(3),
					Revision:            int64ptr(100),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.3:8080",
					ClientURL:           "https://10.0.0.3:8081",
					Self:                false,
				},
			},
		},
		{
			label: "node0 status lookup failed",
			args:  args,
			etcdutilResponses: &mockEtcdutil{
				statusResponse:       nil,
				listMembersError:     nil,
				healthCheckResponse:  nil,
				listMembersEndpoints: []string{},
				healthCheckEndpoints: []string{},
				statusHandlerResponse: []*mockStatusResponse{
					&mockStatusResponse{
						delay: 100 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint: "https://10.0.0.1:8081",
						},
						err: errors.New("failed"),
					},
					&mockStatusResponse{
						delay: 300 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.2:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(2),
							LeaderID:  uint64ptr(2),
							Revision:  int64ptr(101),
						},
						err: nil,
					},
					&mockStatusResponse{
						delay: 200 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.3:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(3),
							LeaderID:  uint64ptr(2),
							Revision:  int64ptr(100),
						},
						err: nil,
					},
				},
				listMembersResponse: []*etcdutil.MemberResp{
					&etcdutil.MemberResp{
						ID: uint64ptr(1),
						PeerURLs: []string{
							"https://10.0.0.1:8080",
						},
					},
					&etcdutil.MemberResp{
						ID: uint64ptr(2),
						PeerURLs: []string{
							"https://10.0.0.2:8080",
						},
					},
					&etcdutil.MemberResp{
						ID: uint64ptr(3),
						PeerURLs: []string{
							"https://10.0.0.3:8080",
						},
					},
				},
			},
			expectedHealthy:         true,
			expectedClusterID:       uint64ptr(10),
			expectedLeaderID:        uint64ptr(2),
			expectedRevision:        int64ptr(101),
			expectedHealthEndpoints: []string{"https://10.0.0.2:8081", "https://10.0.0.3:8081"},
			expectedFinalEndpoints:  []string{"https://10.0.0.1:8081", "https://10.0.0.2:8081", "https://10.0.0.3:8081"},
			expectedMemberMap: map[string]*Member{
				"node0": &Member{
					Name:                "node0",
					Healthy:             false,
					MemberID:            nil,
					MemberIDFromCluster: uint64ptr(1),
					Revision:            nil,
					ClusterID:           nil,
					LeaderID:            nil,
					PeerURL:             "https://10.0.0.1:8080",
					ClientURL:           "https://10.0.0.1:8081",
					Self:                true,
				},
				"node1": &Member{
					Name:                "node1",
					Healthy:             true,
					MemberID:            uint64ptr(2),
					MemberIDFromCluster: uint64ptr(2),
					Revision:            int64ptr(101),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(2),
					PeerURL:             "https://10.0.0.2:8080",
					ClientURL:           "https://10.0.0.2:8081",
					Self:                false,
				},
				"node2": &Member{
					Name:                "node2",
					Healthy:             true,
					MemberID:            uint64ptr(3),
					MemberIDFromCluster: uint64ptr(3),
					Revision:            int64ptr(100),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(2),
					PeerURL:             "https://10.0.0.3:8080",
					ClientURL:           "https://10.0.0.3:8081",
					Self:                false,
				},
			},
		},
		{
			label: "all status lookup failed",
			args:  args,
			etcdutilResponses: &mockEtcdutil{
				statusResponse:       nil,
				listMembersError:     nil,
				healthCheckResponse:  nil,
				listMembersEndpoints: []string{},
				healthCheckEndpoints: []string{},
				statusHandlerResponse: []*mockStatusResponse{
					&mockStatusResponse{
						delay: 100 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint: "https://10.0.0.1:8081",
						},
						err: errors.New("failed"),
					},
					&mockStatusResponse{
						delay: 300 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint: "https://10.0.0.2:8081",
						},
						err: errors.New("failed"),
					},
					&mockStatusResponse{
						delay: 200 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint: "https://10.0.0.3:8081",
						},
						err: errors.New("failed"),
					},
				},
				listMembersResponse: []*etcdutil.MemberResp{
					&etcdutil.MemberResp{
						ID: uint64ptr(2),
						PeerURLs: []string{
							"https://10.0.0.2:8080",
						},
					},
					&etcdutil.MemberResp{
						ID: uint64ptr(3),
						PeerURLs: []string{
							"https://10.0.0.3:8080",
						},
					},
				},
			},
			expectedHealthy:         false,
			expectedClusterID:       nil,
			expectedLeaderID:        nil,
			expectedRevision:        nil,
			expectedHealthEndpoints: []string{},
			expectedFinalEndpoints:  []string{},
			expectedMemberMap: map[string]*Member{
				"node0": &Member{
					Name:                "node0",
					Healthy:             false,
					MemberID:            nil,
					MemberIDFromCluster: nil,
					Revision:            nil,
					ClusterID:           nil,
					LeaderID:            nil,
					PeerURL:             "https://10.0.0.1:8080",
					ClientURL:           "https://10.0.0.1:8081",
					Self:                true,
				},
				"node1": &Member{
					Name:                "node1",
					Healthy:             false,
					MemberID:            nil,
					MemberIDFromCluster: nil,
					Revision:            nil,
					ClusterID:           nil,
					LeaderID:            nil,
					PeerURL:             "https://10.0.0.2:8080",
					ClientURL:           "https://10.0.0.2:8081",
					Self:                false,
				},
				"node2": &Member{
					Name:                "node2",
					Healthy:             false,
					MemberID:            nil,
					MemberIDFromCluster: nil,
					Revision:            nil,
					ClusterID:           nil,
					LeaderID:            nil,
					PeerURL:             "https://10.0.0.3:8080",
					ClientURL:           "https://10.0.0.3:8081",
					Self:                false,
				},
			},
		},
		{
			label: "list members lookup failed",
			args:  args,
			etcdutilResponses: &mockEtcdutil{
				statusResponse:       nil,
				listMembersError:     errors.New("failed"),
				healthCheckResponse:  nil,
				listMembersEndpoints: []string{},
				healthCheckEndpoints: []string{},
				statusHandlerResponse: []*mockStatusResponse{
					&mockStatusResponse{
						delay: 100 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.1:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(1),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(99),
						},
						err: nil,
					},
					&mockStatusResponse{
						delay: 300 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.2:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(2),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(101),
						},
						err: nil,
					},
					&mockStatusResponse{
						delay: 200 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.3:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(3),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(100),
						},
						err: nil,
					},
				},
				listMembersResponse: []*etcdutil.MemberResp{},
			},
			expectedHealthy:         false,
			expectedClusterID:       uint64ptr(10),
			expectedLeaderID:        nil,
			expectedRevision:        nil,
			expectedHealthEndpoints: []string{"https://10.0.0.1:8081", "https://10.0.0.2:8081", "https://10.0.0.3:8081"},
			expectedFinalEndpoints:  []string{},
			expectedMemberMap: map[string]*Member{
				"node0": &Member{
					Name:                "node0",
					Healthy:             false,
					MemberID:            uint64ptr(1),
					MemberIDFromCluster: nil,
					Revision:            int64ptr(99),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.1:8080",
					ClientURL:           "https://10.0.0.1:8081",
					Self:                true,
				},
				"node1": &Member{
					Name:                "node1",
					Healthy:             false,
					MemberID:            uint64ptr(2),
					MemberIDFromCluster: nil,
					Revision:            int64ptr(101),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.2:8080",
					ClientURL:           "https://10.0.0.2:8081",
					Self:                false,
				},
				"node2": &Member{
					Name:                "node2",
					Healthy:             false,
					MemberID:            uint64ptr(3),
					MemberIDFromCluster: nil,
					Revision:            int64ptr(100),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.3:8080",
					ClientURL:           "https://10.0.0.3:8081",
					Self:                false,
				},
			},
		},
		{
			label: "health check failed",
			args:  args,
			etcdutilResponses: &mockEtcdutil{
				statusResponse:       nil,
				listMembersError:     nil,
				healthCheckResponse:  errors.New("failed"),
				listMembersEndpoints: []string{},
				healthCheckEndpoints: []string{},
				statusHandlerResponse: []*mockStatusResponse{
					&mockStatusResponse{
						delay: 100 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.1:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(1),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(99),
						},
						err: nil,
					},
					&mockStatusResponse{
						delay: 300 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.2:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(2),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(101),
						},
						err: nil,
					},
					&mockStatusResponse{
						delay: 200 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.3:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(3),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(100),
						},
						err: nil,
					},
				},
				listMembersResponse: []*etcdutil.MemberResp{},
			},
			expectedHealthy:         false,
			expectedClusterID:       uint64ptr(10),
			expectedLeaderID:        nil,
			expectedRevision:        nil,
			expectedHealthEndpoints: []string{},
			expectedFinalEndpoints:  []string{},
			expectedMemberMap: map[string]*Member{
				"node0": &Member{
					Name:                "node0",
					Healthy:             false,
					MemberID:            uint64ptr(1),
					MemberIDFromCluster: nil,
					Revision:            int64ptr(99),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.1:8080",
					ClientURL:           "https://10.0.0.1:8081",
					Self:                true,
				},
				"node1": &Member{
					Name:                "node1",
					Healthy:             false,
					MemberID:            uint64ptr(2),
					MemberIDFromCluster: nil,
					Revision:            int64ptr(101),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.2:8080",
					ClientURL:           "https://10.0.0.2:8081",
					Self:                false,
				},
				"node2": &Member{
					Name:                "node2",
					Healthy:             false,
					MemberID:            uint64ptr(3),
					MemberIDFromCluster: nil,
					Revision:            int64ptr(100),
					ClusterID:           uint64ptr(10),
					LeaderID:            uint64ptr(1),
					PeerURL:             "https://10.0.0.3:8080",
					ClientURL:           "https://10.0.0.3:8081",
					Self:                false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			status, _ := New(tt.args)
			status.UpdateFromStatus(tt.args, tt.etcdutilResponses)
			status.SetMembersHealth()

			assert.Equal(t, tt.expectedHealthy, status.Healthy)
			assert.Equal(t, tt.expectedClusterID, status.ClusterID)
			assert.Equal(t, tt.expectedLeaderID, status.LeaderID)
			assert.Equal(t, tt.expectedRevision, status.Revision)
			assert.Equal(t, tt.expectedHealthEndpoints, tt.etcdutilResponses.healthCheckEndpoints)
			assert.Equal(t, tt.expectedFinalEndpoints, status.healthyClientURLs)
			assert.Equal(t, tt.expectedMemberMap, status.MemberMap)
			assert.Equal(t, true, status.MemberSelf.Self)
		})
	}
}
