package status

import (
	"errors"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/manifest"
	"github.com/randomcoww/etcd-wrapper/pkg/s3util"
	"github.com/stretchr/testify/assert"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

type mockStep struct {
	mockClient                      *etcdutil.MockClient
	expectedMemberState             MemberState
	expectedCreateClusterState      string
	expectedNodeReplacementPeerURLs []string
	expectedMemberAddedID           uint64
	expectedMemberRemovedID         uint64
	expectedEndpoints               []string
}

func TestStateMachineRun(t *testing.T) {

	// NODE0
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

	node0MemberWithAdditionalPeer := &etcdserverpb.Member{
		ID:   1001,
		Name: "node0",
		PeerURLs: []string{
			"https://10.0.0.1:8001",
			"https://10.0.1.1:8001",
		},
		ClientURLs: []string{
			"https://10.0.0.1:8081",
		},
	}
	newNode0Member := &etcdserverpb.Member{
		ID:   1004,
		Name: "",
		PeerURLs: []string{
			"https://10.0.0.1:8001",
			"https://10.0.1.1:8001",
		},
		ClientURLs: []string{},
	}
	replacedNode0Member := &etcdserverpb.Member{
		ID:   1004,
		Name: "node0",
		PeerURLs: []string{
			"https://10.0.0.1:8001",
			"https://10.0.1.1:8001",
		},
		ClientURLs: []string{
			"https://10.0.0.4:8081",
		},
	}
	replacedNode0StatusResponse := &etcdserverpb.StatusResponse{
		Header: &etcdserverpb.ResponseHeader{
			MemberId: replacedNode0Member.ID,
		},
		Leader: replacedNode0Member.ID,
	}

	// NODE1
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

	newNode1Member := &etcdserverpb.Member{
		ID:   1005,
		Name: "",
		PeerURLs: []string{
			"https://10.0.0.2:8001",
		},
		ClientURLs: []string{},
	}
	replacedNode1Member := &etcdserverpb.Member{
		ID:   1005,
		Name: "node1",
		PeerURLs: []string{
			"https://10.0.0.2:8001",
		},
		ClientURLs: []string{
			"https://10.0.0.5:8081",
		},
	}
	replacedNode1StatusResponse := &etcdserverpb.StatusResponse{
		Header: &etcdserverpb.ResponseHeader{
			MemberId: replacedNode1Member.ID,
		},
		Leader: happyNode0Member.ID,
	}

	// NODE2
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
		HealthCheckErr: errors.New("unhealty"),
		MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
			Err: errors.New("missing list"),
		},
		StatusResponseWithErr: map[string]*etcdutil.StatusResponseWithErr{},
	}

	clientResponseNode0Down := &etcdutil.MockClient{
		MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
			Members: []*etcdserverpb.Member{
				node0MemberWithAdditionalPeer,
				happyNode1Member,
				happyNode2Member,
			},
		},
		MemberRemoveResponseWithErr: &etcdutil.MemberListResponseWithErr{
			Members: []*etcdserverpb.Member{
				happyNode1Member,
				happyNode2Member,
			},
		},
		MemberAddResponseWithErr: &etcdutil.MemberAddResponseWithErr{
			Members: []*etcdserverpb.Member{
				newNode0Member,
				happyNode1Member,
				happyNode2Member,
			},
			Member: newNode0Member,
		},
		StatusResponseWithErr: map[string]*etcdutil.StatusResponseWithErr{
			"https://10.0.0.2:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode1StatusResponse,
			},
			"https://10.0.0.3:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode2StatusResponse,
			},
		},
	}
	clientResponseNode0Replaced := &etcdutil.MockClient{
		MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
			Members: []*etcdserverpb.Member{
				replacedNode0Member,
				happyNode1Member,
				happyNode2Member,
			},
		},
		StatusResponseWithErr: map[string]*etcdutil.StatusResponseWithErr{
			"https://10.0.0.4:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: replacedNode0StatusResponse,
			},
			"https://10.0.0.2:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode1StatusResponse,
			},
			"https://10.0.0.3:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode2StatusResponse,
			},
		},
	}

	clientResponseNode1Down := &etcdutil.MockClient{
		MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
			Members: []*etcdserverpb.Member{
				happyNode0Member,
				happyNode1Member,
				happyNode2Member,
			},
		},
		MemberRemoveResponseWithErr: &etcdutil.MemberListResponseWithErr{
			Members: []*etcdserverpb.Member{
				happyNode0Member,
				happyNode2Member,
			},
		},
		MemberAddResponseWithErr: &etcdutil.MemberAddResponseWithErr{
			Members: []*etcdserverpb.Member{
				happyNode0Member,
				newNode1Member,
				happyNode2Member,
			},
			Member: newNode1Member,
		},
		StatusResponseWithErr: map[string]*etcdutil.StatusResponseWithErr{
			"https://10.0.0.1:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode0StatusResponse,
			},
			"https://10.0.0.3:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode2StatusResponse,
			},
		},
	}
	clientResponseNode1Replaced := &etcdutil.MockClient{
		MemberListResponseWithErr: &etcdutil.MemberListResponseWithErr{
			Members: []*etcdserverpb.Member{
				happyNode0Member,
				replacedNode1Member,
				happyNode2Member,
			},
		},
		StatusResponseWithErr: map[string]*etcdutil.StatusResponseWithErr{
			"https://10.0.0.1:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode0StatusResponse,
			},
			"https://10.0.0.5:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: replacedNode1StatusResponse,
			},
			"https://10.0.0.3:8081": &etcdutil.StatusResponseWithErr{
				StatusResponse: happyNode2StatusResponse,
			},
		},
	}

	tests := []struct {
		label              string
		args               *arg.Args
		initialMemberState MemberState
		initialEndpoints   []string
		mockSteps          []*mockStep
	}{
		{
			label:              "happy path",
			initialMemberState: MemberStateHealthy,
			initialEndpoints: []string{
				"https://10.0.0.1:8081",
				"https://10.0.0.2:8081",
				"https://10.0.0.3:8081",
			},
			mockSteps: []*mockStep{
				&mockStep{
					mockClient:          clientResponseHealthy,
					expectedMemberState: MemberStateHealthy,
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
			label:              "init -> healthy",
			initialMemberState: MemberStateInit,
			initialEndpoints: []string{
				"https://10.0.0.1:8081",
			},
			mockSteps: []*mockStep{
				&mockStep{
					mockClient:          clientResponseHealthy,
					expectedMemberState: MemberStateHealthy,
					expectedEndpoints: []string{
						"https://10.0.0.1:8081",
						"https://10.0.0.2:8081",
						"https://10.0.0.3:8081",
					},
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
			label:              "init -> attempt to join existing -> create new -> healthy",
			initialMemberState: MemberStateInit,
			initialEndpoints: []string{
				"https://10.0.0.1:8081",
			},
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
					expectedEndpoints: []string{
						"https://10.0.0.1:8081",
						"https://10.0.0.2:8081",
						"https://10.0.0.3:8081",
					},
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
			label:              "init -> attempt to join existing -> healthy",
			initialMemberState: MemberStateInit,
			initialEndpoints: []string{
				"https://10.0.0.1:8081",
			},
			mockSteps: []*mockStep{
				&mockStep{
					mockClient:                 clientResponseNoCluster,
					expectedMemberState:        MemberStateHealthy,
					expectedCreateClusterState: "existing",
				},
				&mockStep{
					mockClient:          clientResponseHealthy,
					expectedMemberState: MemberStateHealthy,
					expectedEndpoints: []string{
						"https://10.0.0.1:8081",
						"https://10.0.0.2:8081",
						"https://10.0.0.3:8081",
					},
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
			label:              "healthy -> bad local node status -> healthy",
			initialMemberState: MemberStateHealthy,
			initialEndpoints: []string{
				"https://10.0.0.1:8081",
				"https://10.0.0.2:8081",
				"https://10.0.0.3:8081",
			},
			mockSteps: []*mockStep{
				&mockStep{
					mockClient:          clientResponseNode0Down,
					expectedMemberState: MemberStateHealthy,
				},
				&mockStep{
					mockClient:          clientResponseNode0Down,
					expectedMemberState: MemberStateFailed,
				},
				&mockStep{
					mockClient:                      clientResponseNode0Down,
					expectedMemberState:             MemberStateHealthy,
					expectedMemberRemovedID:         0,
					expectedMemberAddedID:           0,
					expectedCreateClusterState:      "existing",
					expectedNodeReplacementPeerURLs: []string{},
				},
				&mockStep{
					mockClient:          clientResponseNode0Replaced,
					expectedMemberState: MemberStateHealthy,
					expectedEndpoints: []string{
						"https://10.0.0.4:8081",
						"https://10.0.0.2:8081",
						"https://10.0.0.3:8081",
					},
				},
				&mockStep{
					mockClient:          clientResponseNode0Replaced,
					expectedMemberState: MemberStateFailed,
				},
				&mockStep{
					mockClient:          clientResponseNode0Replaced,
					expectedMemberState: MemberStateHealthy,
				},
				&mockStep{
					mockClient:          clientResponseNode0Replaced,
					expectedMemberState: MemberStateHealthy,
				},
			},
		},
		{
			label:              "healthy -> bad remote node status -> healthy",
			initialMemberState: MemberStateHealthy,
			initialEndpoints: []string{
				"https://10.0.0.1:8081",
				"https://10.0.0.2:8081",
				"https://10.0.0.3:8081",
			},
			mockSteps: []*mockStep{
				&mockStep{
					mockClient:          clientResponseNode1Down,
					expectedMemberState: MemberStateHealthy,
				},
				&mockStep{
					mockClient:              clientResponseNode1Down,
					expectedMemberState:     MemberStateHealthy,
					expectedMemberRemovedID: happyNode1Member.ID,
					expectedMemberAddedID:   newNode1Member.ID,
				},
				&mockStep{
					mockClient:          clientResponseNode1Replaced,
					expectedMemberState: MemberStateHealthy,
					expectedEndpoints: []string{
						"https://10.0.0.1:8081",
						"https://10.0.0.5:8081",
						"https://10.0.0.3:8081",
					},
				},
				&mockStep{
					mockClient:          clientResponseNode1Replaced,
					expectedMemberState: MemberStateHealthy,
				},
				&mockStep{
					mockClient:          clientResponseNode1Replaced,
					expectedMemberState: MemberStateHealthy,
				},
			},
		},
		{
			label:              "healthy -> cluster down -> healthy",
			initialMemberState: MemberStateHealthy,
			initialEndpoints: []string{
				"https://10.0.0.1:8081",
				"https://10.0.0.2:8081",
				"https://10.0.0.3:8081",
			},
			mockSteps: []*mockStep{
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
					expectedEndpoints: []string{
						"https://10.0.0.1:8081",
						"https://10.0.0.2:8081",
						"https://10.0.0.3:8081",
					},
				},
				&mockStep{
					mockClient:          clientResponseHealthy,
					expectedMemberState: MemberStateHealthy,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			args := &arg.Args{
				Name: "node0",
				EtcdPod: &v1.Pod{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Pod",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "etcd-pod",
						Namespace: "test-ns",
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "etcd",
								Image: "etcd-image:latest",
							},
						},
					},
				},
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
				Endpoints:       tt.initialEndpoints,
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
				if len(step.expectedNodeReplacementPeerURLs) > 0 {
					assert.ElementsMatch(t, step.expectedNodeReplacementPeerURLs, args.ListenPeerURLs)
					assert.ElementsMatch(t, step.expectedNodeReplacementPeerURLs, args.InitialAdvertisePeerURLs)
				}
				assert.Equal(t, step.expectedMemberAddedID, step.mockClient.MemberAddedID)
				assert.Equal(t, step.expectedMemberRemovedID, step.mockClient.MemberRemovedID)
				if len(step.expectedEndpoints) > 0 {
					assert.ElementsMatch(t, step.expectedEndpoints, status.Endpoints)
				}
			}
			status.Quit <- struct{}{}
		})
	}
}
