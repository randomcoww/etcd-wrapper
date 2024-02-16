package status

import (
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewStatus(t *testing.T) {
	tests := []struct {
		label      string
		args       *arg.Args
		mockClient *etcdutil.MockClient
	}{
		{
			label: "default",
			args: &arg.Args{
				AdvertiseClientURLs: []string{
					"https://10.0.0.1:8081",
					"https://127.0.0.1:8081",
				},
			},
			mockClient: &etcdutil.MockClient{
				EndpointsResponse: []string{
					"https://10.0.0.1:8081",
					"https://127.0.0.1:8081",
				},
				SyncEndpointsErr: nil,
				StatusResponses: []*etcdutil.MockStatus{
					&etcdutil.MockStatus{
						Header: &etcdutil.MockHeader{
							ClusterId: 3001,
							MemberId:  1001,
							Revision:  5000,
						},
						Leader:    1001,
						RaftIndex: 2001,
						IsLearner: false,
						Err:       nil,
					},
				},
				ListMembersReponse: &etcdutil.MockList{
					Header: &etcdutil.MockHeader{
						ClusterId: 3001,
						MemberId:  1001,
						Revision:  5000,
					},
					Members: []*etcdutil.MockMember{
						&etcdutil.MockMember{
							ID: 1001,
							PeerURLs: []string{
								"https//10.0.0.1:8001",
								"https//10.0.0.2:8001",
								"https//10.0.0.3:8001",
							},
							IsLearner: false,
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
		})
	}
}
