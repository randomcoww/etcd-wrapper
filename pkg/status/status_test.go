package status

import (
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	"github.com/stretchr/testify/assert"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"testing"
)

func TestNewStatus(t *testing.T) {
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
				EndpointsResponse: []string{
					"https://10.0.0.1:8081",
					"https://127.0.0.1:8081",
				},
				SyncEndpointsErr: nil,
				StatusResponses: []struct {
					*etcdserverpb.StatusResponse
					Err error
				}{
					struct {
						*etcdserverpb.StatusResponse
						Err error
					}{
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
					struct {
						*etcdserverpb.StatusResponse
						Err error
					}{
						StatusResponse: &etcdserverpb.StatusResponse{
							Header: &etcdserverpb.ResponseHeader{
								ClusterId: 3001,
								MemberId:  1002,
								Revision:  5001,
							},
							Leader:    1001,
							RaftIndex: 2001,
							IsLearner: false,
						},
						Err: nil,
					},
					struct {
						*etcdserverpb.StatusResponse
						Err error
					}{
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
				},
				MemberListResponse: struct {
					*etcdserverpb.MemberListResponse
					Err error
				}{
					MemberListResponse: &etcdserverpb.MemberListResponse{
						Header: &etcdserverpb.ResponseHeader{
							ClusterId: 3001,
							MemberId:  1001,
							Revision:  5000,
						},
						Members: []*etcdserverpb.Member{
							&etcdserverpb.Member{
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
							&etcdserverpb.Member{
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
							&etcdserverpb.Member{
								ID:   1003,
								Name: "node2",
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
						},
					},
					Err: nil,
				},
			},
			expectedSelf: &Member{
				Status: &etcdserverpb.StatusResponse{
					Header: &etcdserverpb.ResponseHeader{
						ClusterId: 3001,
						MemberId:  1001,
						Revision:  5000,
					},
					Leader:    1001,
					RaftIndex: 2001,
					IsLearner: false,
				},
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
				},
			},
			expectedLeader: &Member{
				Status: &etcdserverpb.StatusResponse{
					Header: &etcdserverpb.ResponseHeader{
						ClusterId: 3001,
						MemberId:  1001,
						Revision:  5000,
					},
					Leader:    1001,
					RaftIndex: 2001,
					IsLearner: false,
				},
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
				},
			},
			expectedMemberMap: map[uint64]*Member{
				1001: &Member{
					Status: &etcdserverpb.StatusResponse{
						Header: &etcdserverpb.ResponseHeader{
							ClusterId: 3001,
							MemberId:  1001,
							Revision:  5000,
						},
						Leader:    1001,
						RaftIndex: 2001,
						IsLearner: false,
					},
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
					},
				},
				1002: &Member{
					Status: &etcdserverpb.StatusResponse{
						Header: &etcdserverpb.ResponseHeader{
							ClusterId: 3001,
							MemberId:  1002,
							Revision:  5001,
						},
						Leader:    1001,
						RaftIndex: 2001,
						IsLearner: false,
					},
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
					},
				},
				1003: &Member{
					Status: &etcdserverpb.StatusResponse{
						Header: &etcdserverpb.ResponseHeader{
							ClusterId: 3001,
							MemberId:  1003,
							Revision:  5000,
						},
						Leader:    1001,
						RaftIndex: 2001,
						IsLearner: false,
					},
					Member: &etcdserverpb.Member{
						ID:   1003,
						Name: "node2",
						PeerURLs: []string{
							"https//10.0.0.3:8001",
							"https//127.0.0.1:8001",
						},
						ClientURLs: []string{
							"https//10.0.0.3:8081",
							"https//127.0.0.1:8081",
						},
					},
				},
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
			assert.Equal(t, tt.expectedLeader, status.Leader)
			assert.Equal(t, tt.expectedMemberMap, status.MemberMap)
		})
	}
}
