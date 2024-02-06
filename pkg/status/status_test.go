package status

import (
	"crypto/tls"
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
}

type mockEtcdutil struct {
	statusResponse        error
	statusHandlerResponse []*mockStatusResponse
	listMembersResponse   []*etcdutil.MemberResp
	listMembersError      error
	healthCheckResponse   error
}

func (v *mockEtcdutil) Status(endpoints []string, handler func(*etcdutil.StatusResp), tlsConfig *tls.Config) error {
	if v.statusResponse != nil {
		return v.statusResponse
	}
	var wg sync.WaitGroup
	for _, status := range v.statusHandlerResponse {
		wg.Add(1)
		go func(status *mockStatusResponse) {
			time.Sleep(status.delay)
			handler(status.resp)
			wg.Done()
		}(status)
	}
	wg.Wait()
	return nil
}

func (v *mockEtcdutil) ListMembers(endpoints []string, tlsConfig *tls.Config) ([]*etcdutil.MemberResp, error) {
	return v.listMembersResponse, v.listMembersError
}

func (v *mockEtcdutil) HealthCheck(endpoints []string, tlsConfig *tls.Config) error {
	return v.healthCheckResponse
}

func TestSyncStatus(t *testing.T) {
	tests := []struct {
		label                 string
		name                  string
		initialCluster        string
		initialClusterClients string
		etcdutilResponses     *mockEtcdutil
		expectedHealthy       bool
		expectedClusterID     uint64
		expectedLeaderID      uint64
		expectedMemberMap     map[string]*Member
		expectedMemberSelf    *Member
	}{
		{
			label:                 "happy path",
			name:                  "node0",
			initialCluster:        "node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080,node2=https://10.0.0.3:8080",
			initialClusterClients: "node0=https://10.0.0.1:8081,node1=https://10.0.0.2:8081,node2=https://10.0.0.3:8081",
			etcdutilResponses: &mockEtcdutil{
				statusResponse:      nil,
				listMembersError:    nil,
				healthCheckResponse: nil,
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
				statusHandlerResponse: []*mockStatusResponse{
					&mockStatusResponse{
						delay: 100 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.1:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(1),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(100),
						},
					},
					&mockStatusResponse{
						delay: 300 * time.Millisecond,
						resp: &etcdutil.StatusResp{
							Endpoint:  "https://10.0.0.2:8081",
							ClusterID: uint64ptr(10),
							MemberID:  uint64ptr(2),
							LeaderID:  uint64ptr(1),
							Revision:  int64ptr(100),
						},
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
					},
				},
			},
			expectedHealthy:   true,
			expectedClusterID: uint64(10),
			expectedLeaderID:  uint64(1),
			expectedMemberMap: map[string]*Member{
				"node0": &Member{
					Name:                "node0",
					Healthy:             true,
					MemberID:            uint64ptr(1),
					MemberIDFromCluster: uint64ptr(1),
					Revision:            int64ptr(100),
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
					Revision:            int64ptr(100),
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
			expectedMemberSelf: &Member{
				Name:                "node0",
				Healthy:             true,
				MemberID:            uint64ptr(1),
				MemberIDFromCluster: uint64ptr(1),
				Revision:            int64ptr(100),
				ClusterID:           uint64ptr(10),
				LeaderID:            uint64ptr(1),
				PeerURL:             "https://10.0.0.1:8080",
				ClientURL:           "https://10.0.0.1:8081",
				Self:                true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			v := &Status{}
			v.populateMembersFromInitialCluster(tt.name, tt.initialCluster, tt.initialClusterClients)

			v.SyncStatus(tt.etcdutilResponses)
			assert.Equal(t, true, v.Healthy)
			assert.Equal(t, tt.expectedClusterID, *v.ClusterID)
			assert.Equal(t, tt.expectedLeaderID, *v.LeaderID)
			assert.Equal(t, tt.expectedMemberMap, v.MemberMap)
			assert.Equal(t, tt.expectedMemberSelf, v.MemberSelf)
		})
	}
}
