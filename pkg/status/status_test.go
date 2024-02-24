package status

import (
	"errors"
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

	happyNode0 := &etcdutil.MockNode{
		Member: &etcdserverpb.Member{
			ID:   1001,
			Name: "node0",
			PeerURLs: []string{
				"https://10.0.0.1:8001",
			},
			ClientURLs: []string{
				"https://10.0.0.1:8081",
			},
			IsLearner: false,
		},
		StatusResponse: &etcdserverpb.StatusResponse{
			Header: &etcdserverpb.ResponseHeader{
				ClusterId: 3001,
				MemberId:  1001,
				Revision:  5000,
			},
			Leader:    1001,
			RaftIndex: 2001,
			IsLearner: false,
		},
		StatusErr: nil,
	}

	unhealthyNode0 := &etcdutil.MockNode{
		Member: &etcdserverpb.Member{
			ID:   1001,
			Name: "node0",
			PeerURLs: []string{
				"https://10.0.0.1:8001",
			},
			ClientURLs: []string{
				"https://10.0.0.1:8081",
			},
			IsLearner: false,
		},
		StatusResponse: nil,
		StatusErr:      errors.New("bad status"),
	}

	happyNode1 := &etcdutil.MockNode{
		Member: &etcdserverpb.Member{
			ID:   1002,
			Name: "node1",
			PeerURLs: []string{
				"https://10.0.0.2:8001",
			},
			ClientURLs: []string{
				"https://10.0.0.2:8081",
			},
			IsLearner: false,
		},
		StatusResponse: &etcdserverpb.StatusResponse{
			Header: &etcdserverpb.ResponseHeader{
				ClusterId: 3001,
				MemberId:  1002,
				Revision:  5000,
			},
			Leader:    1001,
			RaftIndex: 2001,
			IsLearner: false,
		},
		StatusErr: nil,
	}

	happyNode2 := &etcdutil.MockNode{
		Member: &etcdserverpb.Member{
			ID:   1003,
			Name: "node2",
			PeerURLs: []string{
				"https://10.0.0.3:8001",
			},
			ClientURLs: []string{
				"https://10.0.0.3:8081",
			},
			IsLearner: false,
		},
		StatusResponse: &etcdserverpb.StatusResponse{
			Header: &etcdserverpb.ResponseHeader{
				ClusterId: 3001,
				MemberId:  1003,
				Revision:  5000,
			},
			Leader:    1001,
			RaftIndex: 2001,
			IsLearner: false,
		},
		StatusErr: nil,
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
				NodeEndpointMap: map[string]*etcdutil.MockNode{
					"https://10.0.0.1:8081":  happyNode0,
					"https://127.0.0.1:8081": happyNode0,
					"https://10.0.0.2:8081":  happyNode1,
					"https://10.0.0.3:8081":  happyNode2,
				},
				EndpointsResponse: []string{
					"https://10.0.0.1:8081",
					"https://127.0.0.1:8081",
					"https://10.0.0.2:8081",
					"https://10.0.0.3:8081",
				},
				MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
					ResponseHeader: &etcdserverpb.ResponseHeader{
						ClusterId: 10,
						MemberId:  1,
						Revision:  20,
						RaftTerm:  30,
					},
					Err: nil,
				},
			},
			expectedClusterHealthy: true,
			expectedSelf: &Member{
				Member:         happyNode0.Member,
				StatusResponse: happyNode0.StatusResponse,
			},
			expectedSelfHealthy: true,
			expectedLeader: &Member{
				Member:         happyNode0.Member,
				StatusResponse: happyNode0.StatusResponse,
			},
			expectedMemberMap: map[uint64]*Member{
				1001: &Member{
					Member:         happyNode0.Member,
					StatusResponse: happyNode0.StatusResponse,
				},
				1002: &Member{
					Member:         happyNode1.Member,
					StatusResponse: happyNode1.StatusResponse,
				},
				1003: &Member{
					Member:         happyNode2.Member,
					StatusResponse: happyNode2.StatusResponse,
				},
			},
			expectedEndpoints: []string{
				"https://10.0.0.1:8081",
				"https://127.0.0.1:8081",
				"https://10.0.0.2:8081",
				"https://10.0.0.3:8081",
			},
		},
		{
			label: "endpoint update",
			args:  newCommonArgs(),
			mockClient: &etcdutil.MockClient{
				NodeEndpointMap: map[string]*etcdutil.MockNode{
					"https://10.0.0.1:8081":  happyNode0,
					"https://127.0.0.1:8081": happyNode0,
					"https://10.0.0.2:8081":  happyNode1,
					"https://10.0.0.3:8081":  happyNode2,
				},
				EndpointsResponse: []string{
					"https://127.0.0.1:8081",
					"https://10.0.0.1:8081",
				},
				MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
					ResponseHeader: &etcdserverpb.ResponseHeader{
						ClusterId: 10,
						MemberId:  1,
						Revision:  20,
						RaftTerm:  30,
					},
					Err: nil,
				},
			},
			expectedClusterHealthy: true,
			expectedSelf: &Member{
				Member:         happyNode0.Member,
				StatusResponse: happyNode0.StatusResponse,
			},
			expectedSelfHealthy: true,
			expectedLeader: &Member{
				Member:         happyNode0.Member,
				StatusResponse: happyNode0.StatusResponse,
			},
			expectedMemberMap: map[uint64]*Member{
				1001: &Member{
					Member:         happyNode0.Member,
					StatusResponse: happyNode0.StatusResponse,
				},
				1002: &Member{
					Member:         happyNode1.Member,
					StatusResponse: happyNode1.StatusResponse,
				},
				1003: &Member{
					Member:         happyNode2.Member,
					StatusResponse: happyNode2.StatusResponse,
				},
			},
			expectedEndpoints: []string{
				"https://10.0.0.1:8081",
				"https://127.0.0.1:8081",
				"https://10.0.0.2:8081",
				"https://10.0.0.3:8081",
			},
		},
		{
			label: "node0 status error",
			args:  newCommonArgs(),
			mockClient: &etcdutil.MockClient{
				NodeEndpointMap: map[string]*etcdutil.MockNode{
					"https://10.0.0.1:8081": unhealthyNode0,
					"https://10.0.0.2:8081": happyNode1,
					"https://10.0.0.3:8081": happyNode2,
				},
				EndpointsResponse: []string{
					"https://10.0.0.1:8081",
					"https://10.0.0.2:8081",
					"https://10.0.0.3:8081",
				},
				MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
					ResponseHeader: &etcdserverpb.ResponseHeader{
						ClusterId: 10,
						MemberId:  1,
						Revision:  20,
						RaftTerm:  30,
					},
					Err: nil,
				},
			},
			expectedClusterHealthy: true,
			expectedSelf:           nil,
			expectedSelfHealthy:    false,
			expectedLeader: &Member{
				Member:         happyNode0.Member,
				StatusResponse: nil,
			},
			expectedMemberMap: map[uint64]*Member{
				1001: &Member{
					Member:         unhealthyNode0.Member,
					StatusResponse: nil,
				},
				1002: &Member{
					Member:         happyNode1.Member,
					StatusResponse: happyNode1.StatusResponse,
				},
				1003: &Member{
					Member:         happyNode2.Member,
					StatusResponse: happyNode2.StatusResponse,
				},
			},
			expectedEndpoints: []string{
				"https://10.0.0.1:8081",
				"https://10.0.0.2:8081",
				"https://10.0.0.3:8081",
			},
		},
		{
			label: "members list error",
			args:  newCommonArgs(),
			mockClient: &etcdutil.MockClient{
				NodeEndpointMap: map[string]*etcdutil.MockNode{
					"https://10.0.0.1:8081": unhealthyNode0,
					"https://10.0.0.2:8081": happyNode1,
					"https://10.0.0.3:8081": happyNode2,
				},
				EndpointsResponse: []string{
					"https://10.0.0.1:8081",
					"https://10.0.0.2:8081",
					"https://10.0.0.3:8081",
				},
				MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
					ResponseHeader: nil,
					Err:            errors.New("members list error"),
				},
			},
			expectedClusterHealthy: false,
			expectedSelf:           nil,
			expectedSelfHealthy:    false,
			expectedLeader:         nil,
			expectedMemberMap:      map[uint64]*Member{},
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
