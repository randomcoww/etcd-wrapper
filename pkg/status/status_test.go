package status

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPopulateInitialMembers(t *testing.T) {
	tests := []struct {
		label                 string
		name                  string
		initialCluster        string
		initialClusterClients string
		memberMapExpected     map[string]*Member
	}{
		{
			label:                 "happy path",
			name:                  "node0",
			initialCluster:        "node0=https://10.0.0.1:8080,node1=https://10.0.0.2:8080,node2=https://10.0.0.3:8080",
			initialClusterClients: "node0=https://10.0.0.1:8081,node1=https://10.0.0.2:8081,node2=https://10.0.0.3:8081",
			memberMapExpected: map[string]*Member{
				"node0": &Member{
					Name:      "node0",
					PeerURL:   "https://10.0.0.1:8080",
					ClientURL: "https://10.0.0.1:8081",
					Self:      true,
				},
				"node1": &Member{
					Name:      "node1",
					PeerURL:   "https://10.0.0.2:8080",
					ClientURL: "https://10.0.0.2:8081",
					Self:      false,
				},
				"node2": &Member{
					Name:      "node2",
					PeerURL:   "https://10.0.0.3:8080",
					ClientURL: "https://10.0.0.3:8081",
					Self:      false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {

			v := &Status{}
			v.populateMembersFromInitialCluster(tt.name, tt.initialCluster, tt.initialClusterClients)
			assert.Equal(t, tt.memberMapExpected, v.MemberMap)
		})
	}
}
