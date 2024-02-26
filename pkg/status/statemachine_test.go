package status

import (
	"errors"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/manifest"
	"github.com/randomcoww/etcd-wrapper/pkg/s3util"
	"github.com/stretchr/testify/assert"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"sync"
	"testing"
	"time"
)

type mockStep struct {
	mockClient                 *etcdutil.MockClient
	expectedMemberState        MemberState
	expectedCreateClusterState string
}

func TestStateMachineRun(t *testing.T) {
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

	clientResponseHealthy := &etcdutil.MockClient{
		EndpointsResponse: []string{
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
		},
		StatusResponseWithErr: map[string]*etcdutil.StatusResponseWithErr{
			"https://10.0.0.1:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode0StatusResponse,
			},
			"https://10.0.0.2:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode1StatusResponse,
			},
			"https://10.0.0.3:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode2StatusResponse,
			},
		},
	}

	clientResponseNoCluster := &etcdutil.MockClient{
		EndpointsResponse: []string{
			"https://10.0.0.1:8081",
			"https://10.0.0.2:8081",
			"https://10.0.0.3:8081",
		},
		HealthCheckErr: errors.New("unhealty"),
		MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
			Err: errors.New("missing list"),
		},
		StatusResponseWithErr: map[string]*etcdutil.StatusResponseWithErr{},
	}

	tests := []struct {
		mu                 sync.Mutex
		label              string
		args               *arg.Args
		initialMemberState MemberState
		mockSteps          []*mockStep
	}{
		{
			label:              "happy path",
			initialMemberState: MemberStateHealthy,
			mockSteps: []*mockStep{
				&mockStep{
					mockClient:          clientResponseHealthy,
					expectedMemberState: MemberStateHealthy,
				},
				&mockStep{
					mockClient:          clientResponseHealthy,
					expectedMemberState: MemberStateHealthy,
				},
			},
		},
		{
			label:              "init -> healthy",
			initialMemberState: MemberStateInit,
			mockSteps: []*mockStep{
				&mockStep{
					mockClient:          clientResponseHealthy,
					expectedMemberState: MemberStateHealthy,
				},
				&mockStep{
					mockClient:          clientResponseHealthy,
					expectedMemberState: MemberStateHealthy,
				},
			},
		},
		{
			label:              "init -> attempt to join existing -> create new -> healthy",
			initialMemberState: MemberStateInit,
			mockSteps: []*mockStep{
				&mockStep{
					mockClient:                 clientResponseNoCluster,
					expectedMemberState:        MemberStateHealthy,
					expectedCreateClusterState: "existing",
				},
				&mockStep{
					mockClient:          clientResponseNoCluster,
					expectedMemberState: MemberStateHealthy,
				},
				&mockStep{
					mockClient:          clientResponseNoCluster,
					expectedMemberState: MemberStateFailed,
				},
				&mockStep{
					mockClient:                 clientResponseNoCluster,
					expectedMemberState:        MemberStateWait,
					expectedCreateClusterState: "new",
				},
				&mockStep{
					mockClient:          clientResponseNoCluster,
					expectedMemberState: MemberStateWait,
				},
				&mockStep{
					mockClient:          clientResponseHealthy,
					expectedMemberState: MemberStateHealthy,
				},
				&mockStep{
					mockClient:          clientResponseHealthy,
					expectedMemberState: MemberStateHealthy,
				},
			},
		},
		{
			label:              "init -> attempt to join existing -> create new",
			initialMemberState: MemberStateInit,
			mockSteps: []*mockStep{
				&mockStep{
					mockClient:                 clientResponseNoCluster,
					expectedMemberState:        MemberStateHealthy,
					expectedCreateClusterState: "existing",
				},
				&mockStep{
					mockClient:          clientResponseNoCluster,
					expectedMemberState: MemberStateHealthy,
				},
				&mockStep{
					mockClient:          clientResponseNoCluster,
					expectedMemberState: MemberStateFailed,
				},
				&mockStep{
					mockClient:                 clientResponseNoCluster,
					expectedMemberState:        MemberStateWait,
					expectedCreateClusterState: "new",
				},
				&mockStep{
					mockClient:          clientResponseNoCluster,
					expectedMemberState: MemberStateWait,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			args := &arg.Args{
				Name: "node0",
				AdvertiseClientURLs: []string{
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
				HealthCheckFailedCountMax: 2,
				ReadyCheckFailedCountMax:  2,
				S3Client:                  &s3util.MockClient{},
			}
			status := &Status{
				MemberState:     tt.initialMemberState,
				EtcdPod:         &manifest.MockEtcdPod{},
				HealthCheckChan: make(chan time.Time),
				BackupChan:      make(chan time.Time),
				Quit:            make(chan struct{}),
			}
			go status.Run(args)

			for _, step := range tt.mockSteps {
				status.NewEtcdClient = func(endpoints []string) (etcdutil.Client, error) {
					step.mockClient.EndpointsResponse = endpoints
					return step.mockClient, nil
				}
				status.HealthCheckChan <- time.Time{}
				time.Sleep(10 * time.Millisecond)
				assert.Equal(t, step.expectedMemberState, status.MemberState)
				if len(step.expectedCreateClusterState) > 0 {
					assert.Equal(t, step.expectedCreateClusterState, args.InitialClusterState)
				}
			}
			status.Quit <- struct{}{}
		})
	}
}
