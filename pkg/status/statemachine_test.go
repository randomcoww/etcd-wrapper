package status

// import (
// 	"errors"
// 	"github.com/randomcoww/etcd-wrapper/pkg/arg"
// 	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
// 	"github.com/stretchr/testify/assert"
// 	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
// 	"testing"
// )

// func TestStateMachineRun(t *testing.T) {
// 	tests := []struct {
// 		label                  string
// 		args                   *arg.Args
// 		mockClient             *etcdutil.MockClient
// 		expectedClusterHealthy bool
// 		expectedSelf           *Member
// 		expectedSelfHealthy    bool
// 		expectedLeader         *Member
// 		expectedMemberMap      map[uint64]*Member
// 		expectedEndpoints      []string
// 	}{
// 		{
// 			label: "init",
// 			args:  *arg.Args{
// 				HealthCheckInterval: 1*time.Millisecond,
// 				BackupInterval: 10*time.Millisecond,
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.label, func(t *testing.T) {
// 			status := &Status{
// 				MemberState: MemberStateInit,
// 				quit: make(chan struct{}, 1),
// 			}

// 			err := status.Run(tt.args, 10)
// 			assert.Equal(t, nil, err)
// 		})
// 	}
// }
