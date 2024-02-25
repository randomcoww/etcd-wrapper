package status

import (
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	"github.com/stretchr/testify/assert"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"testing"
)

func TestSyncStatus(t *testing.T) {
	newCommonArgs := func() *arg.Args {
		return &arg.Args{
			Name: "node0",
			AdvertiseClientURLs: []string{
				"https://127.0.0.1:8081",
				"https://10.0.0.1:8081",
			},
			InitialCluster: []*arg.Node{
				&arg.Node{
					Name:    "node0",
					PeerURL: "https://10.0.0.1:8001",
				},
				&arg.Node{
					Name:    "node1",
					PeerURL: "https://10.0.0.2:8001",
				},
				&arg.Node{
					Name:    "node2",
					PeerURL: "https://10.0.0.3:8001",
				},
			},
		}
	}

	happyNode0Member := &etcdserverpb.Member{
		ID:   1001,
		Name: "node0",
		PeerURLs: []string{
			"https://10.0.0.1:8001",
		},
		ClientURLs: []string{
			"https://10.0.0.1:8081",
		},
	}
	happyNode0StatusResponse := &etcdserverpb.StatusResponse{
		Header: &etcdserverpb.ResponseHeader{
			MemberId: happyNode0Member.ID,
		},
		Leader: happyNode0Member.ID,
	}

	happyNode1Member := &etcdserverpb.Member{
		ID:   1002,
		Name: "node1",
		PeerURLs: []string{
			"https://10.0.0.2:8001",
		},
		ClientURLs: []string{
			"https://10.0.0.2:8081",
		},
	}
	happyNode1StatusResponse := &etcdserverpb.StatusResponse{
		Header: &etcdserverpb.ResponseHeader{
			MemberId: happyNode1Member.ID,
		},
		Leader: happyNode0Member.ID,
	}

	happyNode2Member := &etcdserverpb.Member{
		ID:   1003,
		Name: "node2",
		PeerURLs: []string{
			"https://10.0.0.3:8001",
		},
		ClientURLs: []string{
			"https://10.0.0.3:8081",
		},
	}
	happyNode2StatusResponse := &etcdserverpb.StatusResponse{
		Header: &etcdserverpb.ResponseHeader{
			MemberId: happyNode2Member.ID,
		},
		Leader: happyNode0Member.ID,
	}

	tests := []struct {
		label                  string
		args                   *arg.Args
		mockClient             *etcdutil.MockClient
		expectedClusterHealthy bool
		expectedSelf           *Member
		expectedSelfHealthy    bool
		expectedLeader         *Member
		expectedMemberMap      map[uint64]*Member
		expectedEndpoints      []string
	}{
		{
			label: "happy path",
			args:  newCommonArgs(),
			mockClient: &etcdutil.MockClient{
				EndpointsResponse: []string{
					"https://127.0.0.1:8081",
					"https://10.0.0.1:8081",
					"https://10.0.0.2:8081",
					"https://10.0.0.3:8081",
				},
				MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
					Members: []*etcdserverpb.Member{
						happyNode0Member,
						happyNode1Member,
						happyNode2Member,
					},
					ResponseHeader: &etcdserverpb.ResponseHeader{
						MemberId: happyNode0Member.ID,
					},
					Err: nil,
				},
				StatusResponseWithErr: map[string]*etcdutil.StatusResponseWithErr{
					"https://127.0.0.1:8081": &etcdutil.StatusResponseWithErr{
						StatusResponse: happyNode0StatusResponse,
						Err:            nil,
					},
					"https://10.0.0.1:8081": &etcdutil.StatusResponseWithErr{
						StatusResponse: happyNode0StatusResponse,
						Err:            nil,
					},
					"https://10.0.0.2:8081": &etcdutil.StatusResponseWithErr{
						StatusResponse: happyNode1StatusResponse,
						Err:            nil,
					},
					"https://10.0.0.3:8081": &etcdutil.StatusResponseWithErr{
						StatusResponse: happyNode2StatusResponse,
						Err:            nil,
					},
				},
			},
			expectedClusterHealthy: true,
			expectedSelf: &Member{
				Member:         happyNode0Member,
				StatusResponse: happyNode0StatusResponse,
			},
			expectedSelfHealthy: true,
			expectedLeader: &Member{
				Member:         happyNode0Member,
				StatusResponse: happyNode0StatusResponse,
			},
			expectedMemberMap: map[uint64]*Member{
				1001: &Member{
					Member:         happyNode0Member,
					StatusResponse: happyNode0StatusResponse,
				},
				1002: &Member{
					Member:         happyNode1Member,
					StatusResponse: happyNode1StatusResponse,
				},
				1003: &Member{
					Member:         happyNode2Member,
					StatusResponse: happyNode2StatusResponse,
				},
			},
			expectedEndpoints: []string{
				"https://10.0.0.1:8081",
				"https://10.0.0.2:8081",
				"https://10.0.0.3:8081",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			status := &Status{
				Endpoints: tt.mockClient.EndpointsResponse,
				NewEtcdClient: func(endpoints []string) (etcdutil.Client, error) {
					tt.mockClient.EndpointsResponse = endpoints
					return tt.mockClient, nil
				},
			}
			err := status.SyncStatus(tt.args)
			assert.Equal(t, nil, err)
			assert.Equal(t, tt.expectedClusterHealthy, status.Healthy)
			assert.Equal(t, tt.expectedSelf, status.Self)
			assert.Equal(t, tt.expectedSelfHealthy, status.Self.IsHealthy())
			assert.Equal(t, tt.expectedLeader, status.Leader)
			assert.Equal(t, tt.expectedMemberMap, status.MemberMap)
			assert.ElementsMatch(t, tt.expectedEndpoints, status.Endpoints)
		})
	}
}
