package gossip

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/randomcoww/etcd-wrapper/internal/tlsutil"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

var (
	baseTestPath string = "../../test/outputs"
)

type member struct {
	name     string
	revision int64
	port     int
}

func TestConvergence(t *testing.T) {
	members := []member{
		{
			name:     "node0",
			revision: 1,
			port:     8090,
		},
		{
			name:     "node1",
			revision: 2,
			port:     8091,
		},
		{
			name:     "node2",
			revision: 3,
			port:     8092,
		},
	}

	nodes, callbacks, err := newDummyCluster(t, members)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make([]envelope, len(members))

	for i, node := range nodes {
		wg.Add(1)
		defer node.Stop()

		go func(node *Node, i int) {
			<-callbacks[i]
			results[i] = node.buildEnvelope()
			wg.Done()
		}(node, i)

		time.Sleep(8 * time.Second)

		go node.StartListen()
		go node.StartLoop()
	}

	wg.Wait()
	for i, node := range nodes {
		assert.Equal(t, envelope{
			Name: node.name,
			View: map[string]int64{
				"node0": 1,
				"node1": 2,
				"node2": 3,
			},
			Done: true,
		}, results[i])
	}
}

func newDummyCluster(t *testing.T, members []member) ([]*Node, []chan struct{}, error) {
	logger, _ := zap.NewProduction()

	var done []chan struct{}
	var nodes []*Node
	m := make(map[string]string)
	for _, member := range members {
		m[member.name] = fmt.Sprintf("https://127.0.0.1:%d", member.port)
	}

	for _, member := range members {
		tlsConfig, err := tlsutil.BuildTLSClientConfig(
			filepath.Join(baseTestPath, member.name, "peer", "tls.crt"),
			filepath.Join(baseTestPath, member.name, "peer", "tls.key"),
			[]string{filepath.Join(baseTestPath, "peer", "ca.crt")},
		)
		if err != nil {
			return nil, nil, err
		}
		doneCh := make(chan struct{})
		done = append(done, doneCh)

		node, err := NewNode(logger, member.name, member.revision, m, tlsConfig, doneCh)
		if err != nil {
			return nil, nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, done, nil
}
