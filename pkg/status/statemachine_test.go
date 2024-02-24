package status

import (
	"errors"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	"github.com/randomcoww/etcd-wrapper/pkg/manifest"
	"github.com/randomcoww/etcd-wrapper/pkg/s3util"
	"github.com/stretchr/testify/assert"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"testing"
	"time"
)

func TestStateMachineRun(t *testing.T) {
	commonArgs := &arg.Args{
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
		HealthCheckInterval:       1 * time.Millisecond,
		BackupInterval:            10 * time.Millisecond,
		HealthCheckFailedCountMax: 1,
		ReadyCheckFailedCountMax:  1,
		S3Client:                  &s3util.MockClient{},
	}

	missingNode := &etcdutil.MockNode{
		Member:         nil,
		StatusResponse: nil,
		StatusErr:      errors.New("missing"),
	}

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

	missingLocalNode0 := &etcdutil.MockNode{
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
		StatusResponse: nil,
		StatusErr:      errors.New("missing local"),
	}

	replacedNode0 := &etcdutil.MockNode{
		Member: &etcdserverpb.Member{
			ID:   1001,
			Name: "",
			PeerURLs: []string{
				"https//10.0.0.1:8001",
				"https//127.0.0.1:8001",
			},
			ClientURLs: []string{},
			IsLearner:  false,
		},
		StatusResponse: nil,
		StatusErr:      errors.New("missing"),
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

	respInit := &etcdutil.MockClient{
		NodeEndpointMap: map[string]*etcdutil.MockNode{
			"https://10.0.0.1:8081":  happyNode0,
			"https://127.0.0.1:8081": happyNode0,
			"https://10.0.0.2:8081":  happyNode1,
			"https://10.0.0.3:8081":  happyNode2,
		},
		EndpointsResponse: []string{
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
		MemberRemoveResponseWithErr: &etcdutil.MemberListResponseWithErr{
			ResponseHeader: &etcdserverpb.ResponseHeader{
				ClusterId: 10,
				MemberId:  1,
				Revision:  20,
				RaftTerm:  30,
			},
			Err: nil,
		},
		MemberAddResponseWithErr: &etcdutil.MemberAddResponseWithErr{
			ResponseHeader: &etcdserverpb.ResponseHeader{
				ClusterId: 10,
				MemberId:  1,
				Revision:  20,
				RaftTerm:  30,
			},
			Member: replacedNode0.Member,
			Err:    nil,
		},
	}

	respHealthy := &etcdutil.MockClient{
		NodeEndpointMap: map[string]*etcdutil.MockNode{
			"https://10.0.0.1:8081":  happyNode0,
			"https://127.0.0.1:8081": happyNode0,
			"https://10.0.0.2:8081":  happyNode1,
			"https://10.0.0.3:8081":  happyNode2,
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
		MemberAddResponseWithErr: &etcdutil.MemberAddResponseWithErr{
			ResponseHeader: &etcdserverpb.ResponseHeader{
				ClusterId: 10,
				MemberId:  1,
				Revision:  20,
				RaftTerm:  30,
			},
			Member: &etcdserverpb.Member{},
			Err:    nil,
		},
	}

	respNode0Down := &etcdutil.MockClient{
		NodeEndpointMap: map[string]*etcdutil.MockNode{
			"https://10.0.0.1:8081":  missingLocalNode0,
			"https://127.0.0.1:8081": missingLocalNode0,
			"https://10.0.0.2:8081":  happyNode1,
			"https://10.0.0.3:8081":  happyNode2,
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
		MemberAddResponseWithErr: &etcdutil.MemberAddResponseWithErr{
			ResponseHeader: &etcdserverpb.ResponseHeader{
				ClusterId: 10,
				MemberId:  1,
				Revision:  20,
				RaftTerm:  30,
			},
			Member: &etcdserverpb.Member{},
			Err:    nil,
		},
	}

	respClusterDown := &etcdutil.MockClient{
		NodeEndpointMap: map[string]*etcdutil.MockNode{
			"https://10.0.0.1:8081":  missingNode,
			"https://127.0.0.1:8081": missingNode,
			"https://10.0.0.2:8081":  missingNode,
			"https://10.0.0.3:8081":  missingNode,
		},
		HealthCheckErr: errors.New("health check down"),
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
		MemberAddResponseWithErr: &etcdutil.MemberAddResponseWithErr{
			ResponseHeader: &etcdserverpb.ResponseHeader{
				ClusterId: 10,
				MemberId:  1,
				Revision:  20,
				RaftTerm:  30,
			},
			Member: &etcdserverpb.Member{},
			Err:    nil,
		},
	}

	tests := []struct {
		label                      string
		args                       *arg.Args
		mockClientResponses        *etcdutil.MockClientResponses
		mockEtcdPod                *manifest.MockEtcdPod
		ticks                      int
		initialMemberState         MemberState
		expectedClusterHealthy     bool
		expectedErr                error
		expectedMemberState        MemberState
		expectedMemberMap          map[uint64]*Member
		expectedInitalClusterState string
	}{
		{
			label: "init healthy",
			args:  commonArgs,
			mockClientResponses: &etcdutil.MockClientResponses{
				Resp: []*etcdutil.MockClient{
					respInit,
					respHealthy,
				},
			},
			mockEtcdPod:            &manifest.MockEtcdPod{},
			ticks:                  4,
			initialMemberState:     MemberStateInit,
			expectedClusterHealthy: true,
			expectedErr:            nil,
			expectedMemberState:    MemberStateHealthy,
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
			expectedInitalClusterState: "",
		},
		{
			label: "init node0 down",
			args:  commonArgs,
			mockClientResponses: &etcdutil.MockClientResponses{
				Resp: []*etcdutil.MockClient{
					respInit,
					respNode0Down,
				},
			},
			mockEtcdPod:            &manifest.MockEtcdPod{},
			ticks:                  4,
			initialMemberState:     MemberStateInit,
			expectedClusterHealthy: true,
			expectedErr:            nil,
			expectedMemberState:    MemberStateFailed,
			expectedMemberMap: map[uint64]*Member{
				1001: &Member{
					Member:         happyNode0.Member,
					StatusResponse: replacedNode0.StatusResponse,
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
			expectedInitalClusterState: "existing",
		},
		{
			label: "healthy to cluster down",
			args:  commonArgs,
			mockClientResponses: &etcdutil.MockClientResponses{
				Resp: []*etcdutil.MockClient{
					respClusterDown,
				},
			},
			mockEtcdPod:                &manifest.MockEtcdPod{},
			ticks:                      4,
			initialMemberState:         MemberStateHealthy,
			expectedClusterHealthy:     false,
			expectedErr:                errors.New("Failed ready check"),
			expectedMemberState:        MemberStateWait,
			expectedMemberMap:          map[uint64]*Member{},
			expectedInitalClusterState: "new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			status := &Status{
				Endpoints: tt.mockClientResponses.InitialEndpoints(),
				NewEtcdClient: func(endpoints []string) (etcdutil.Client, error) {
					return tt.mockClientResponses.Next(endpoints), nil
				},
				MemberState: tt.initialMemberState,
				EtcdPod:     tt.mockEtcdPod,
				quit:        make(chan struct{}, 1),
			}
			err := status.Run(tt.args, tt.ticks)
			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, tt.expectedClusterHealthy, status.Healthy)
			assert.Equal(t, tt.expectedMemberState, status.MemberState)
			assert.Equal(t, tt.expectedMemberMap, status.MemberMap)
			assert.Equal(t, tt.expectedInitalClusterState, tt.args.InitialClusterState)
		})
	}
}
