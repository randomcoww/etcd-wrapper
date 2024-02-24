package status

import (
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	"github.com/stretchr/testify/assert"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"testing"
)

func TestNewStatus(t *testing.T) {
	happyNode0 := &etcdutil.MockNode{
		Member: &etcdserverpb.Member{
			ID:   1001,
			Name: "node0",
			PeerURLs: []string{
				"https//10.0.0.1:8001",
				"https//127.0.0.1:8001",
			},
			ClientURLs: []string{
				"https//10.0.0.1:8081",
				"https//127.0.0.1:8081",
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

	happyNode1 := &etcdutil.MockNode{
		Member: &etcdserverpb.Member{
			ID:   1002,
			Name: "node1",
			PeerURLs: []string{
				"https//10.0.0.2:8001",
				"https//127.0.0.1:8001",
			},
			ClientURLs: []string{
				"https//10.0.0.2:8081",
				"https//127.0.0.1:8081",
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
			Name: "node1",
			PeerURLs: []string{
				"https//10.0.0.3:8001",
				"https//127.0.0.1:8001",
			},
			ClientURLs: []string{
				"https//10.0.0.3:8081",
				"https//127.0.0.1:8081",
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

	newMember3 := &etcdserverpb.Member{
		ID:   1004,
		Name: "",
		PeerURLs: []string{
			"https//10.0.0.4:8001",
			"https//127.0.0.1:8001",
		},
		ClientURLs: []string{},
		IsLearner:  false,
	}

	tests := []struct {
		label             string
		args              *arg.Args
		mockClient        *etcdutil.MockClient
		expectedSelf      *Member
		expectedLeader    *Member
		expectedMemberMap map[uint64]*Member
	}{
		{
			label: "happy path",
			args: &arg.Args{
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
			},
			mockClient: &etcdutil.MockClient{
				NodeEndpointMap: map[string]*etcdutil.MockNode{
					"https://10.0.0.1:8081":  happyNode0,
					"https://127.0.0.1:8081": happyNode0,
					"https://10.0.0.2:8081":  happyNode1,
					"https://10.0.0.3:8081":  happyNode2,
				},
				EndpointsResponse: []string{
					"https://10.0.0.1:8081",
					"https://10.0.0.2:8081",
					"https://10.0.0.3:8081",
				},
				SyncEndpointsErr: nil,
				MemberAddResponseWithErr: &etcdutil.MemberAddResponseWithErr{
					ResponseHeader: &etcdserverpb.ResponseHeader{
						ClusterId: 10,
						MemberId:  1,
						Revision:  20,
						RaftTerm:  30,
					},
					Member: newMember3,
					Err:    nil,
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
				MemberRemoveResponseWithErr: &etcdutil.MemberListResponseWithErr{
					ResponseHeader: &etcdserverpb.ResponseHeader{
						ClusterId: 10,
						MemberId:  1,
						Revision:  20,
						RaftTerm:  30,
					},
					Err: nil,
				},
				MemberPromoteResponseWithErr: &etcdutil.MemberListResponseWithErr{
					ResponseHeader: &etcdserverpb.ResponseHeader{
						ClusterId: 10,
						MemberId:  1,
						Revision:  20,
						RaftTerm:  30,
					},
					Err: nil,
				},
				HealthCheckErr:    nil,
				DefragmentErr:     nil,
				CreateSnapshotErr: nil,
			},
			expectedSelf: &Member{
				Member:         happyNode0.Member,
				StatusResponse: happyNode0.StatusResponse,
			},
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			status := &Status{
				Endpoints: tt.mockClient.EndpointsResponse,
				NewEtcdClient: func(endpoints []string) (etcdutil.Util, error) {
					tt.mockClient.EndpointsResponse = endpoints
					return tt.mockClient, nil
				},
			}
			err := status.SyncStatus(tt.args)
			assert.Equal(t, nil, err)
			assert.Equal(t, tt.expectedSelf, status.Self)
			assert.Equal(t, tt.expectedLeader, status.Leader)
			assert.Equal(t, tt.expectedMemberMap, status.MemberMap)
		})
	}
}
