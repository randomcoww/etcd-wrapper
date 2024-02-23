package status

import (
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	"github.com/stretchr/testify/assert"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"testing"
)

func TestNewStatus(t *testing.T) {
	happyNode0 := &etcdutil.Node{
		MemberResponse: &etcdserverpb.Member{
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
		StatusResponseWithErr: &etcdutil.StatusResponseWithErr{
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
			Err: nil,
		},
	}

	happyNode1 := &etcdutil.Node{
		MemberResponse: &etcdserverpb.Member{
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
		StatusResponseWithErr: &etcdutil.StatusResponseWithErr{
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
			Err: nil,
		},
	}

	happyNode2 := &etcdutil.Node{
		MemberResponse: &etcdserverpb.Member{
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
		StatusResponseWithErr: &etcdutil.StatusResponseWithErr{
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
			Err: nil,
		},
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
		label        string
		args         *arg.Args
		mockClient   *etcdutil.MockClient
		expectedSelf *Member
		// expectedLeader    *Member
		// expectedMemberMap map[uint64]*Member
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
					"https://10.0.0.1:8081": happyNode0,
					"https://10.0.0.2:8081": happyNode1,
					"https://10.0.0.3:8081": happyNode2,
				},
				EndpointsResponse: []string{
					"https://10.0.0.1:8081",
					"https://10.0.0.2:8081",
					"https://10.0.0.3:8081",
				},
				SyncEndpointsErr: nil,
				MemberListErr:    nil,
				MemberAddResponseWithErr: &etcdutil.MemberAddResponseWithErr{
					Header: &etcdserverpb.ResponseHeader{
						ClusterId: 10,
						MemberId:  1,
						Revision:  20,
						RaftTerm:  30,
					},
					Member: newMember3,
					Err:    nil,
				},
				MemberRemoveResponseWithErr: &etcdutil.MemberListResponseWithErr{
					Header: &etcdserverpb.ResponseHeader{
						ClusterId: 10,
						MemberId:  1,
						Revision:  20,
						RaftTerm:  30,
					},
					Err: nil,
				},
				MemberPromoteResponseWithErr: &etcdutil.MemberListResponseWithErr{
					Header: &etcdserverpb.ResponseHeader{
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
				Status: happyNode0.MemberResponse,
				Member: happyNode0.StatusResponseWithErr.StatusResponse,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			status := &Status{
				Endpoints: tt.mockClient.EndpointsResponse,
				NewEtcdClient: func(endpoints []string) (etcdutil.Util, error) {
					return tt.mockClient, nil
				},
			}
			err := status.SyncStatus(tt.args)
			assert.Equal(t, nil, err)
			assert.Equal(t, tt.expectedSelf, status.Self)
			// assert.Equal(t, tt.expectedLeader, status.Leader)
			// assert.Equal(t, tt.expectedMemberMap, status.MemberMap)
		})
	}
}
