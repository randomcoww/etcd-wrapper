package gossip

import (
	"fmt"
	"net"
	"path/filepath"
	"sync/atomic"
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
	done := make(chan int)
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

	callbacks := make([]func() error, len(members))
	for i := 0; i < len(members); i++ {
		callbacks[i] = func() error {
			done <- i
			return nil
		}
	}

	nodes, err := newDummyCluster(t, members, callbacks)
	if err != nil {
		t.Fatal(err)
	}

	for _, node := range nodes {
		go node.StartListen()
		go node.StartLoop()
		defer node.Stop()
	}

	count := 0
	for i := range done {
		assert.Equal(t, envelope{
			Name: nodes[i].name,
			View: map[string]int64{
				"node0": 1,
				"node1": 2,
				"node2": 3,
			},
			Done: true,
		}, nodes[i].buildEnvelope())

		count++
		if count >= len(members) {
			close(done)
		}
	}
}

func TestGossipShutdown(t *testing.T) {
	done := make(chan int)
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

	portClosedAtCallback := make([]atomic.Bool, len(members))
	callbacks := make([]func() error, len(members))
	for i := 0; i < len(members); i++ {
		callbacks[i] = func() error {
			c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", members[i].port), 300*time.Millisecond)
			if err != nil {
				portClosedAtCallback[i].Store(true)
			} else {
				c.Close()
			}
			done <- i
			return nil
		}
	}

	nodes, err := newDummyCluster(t, members, callbacks)
	if err != nil {
		t.Fatal(err)
	}

	for _, node := range nodes {
		go node.StartListen()
		go node.StartLoop()
		defer node.Stop()
	}

	count := 0
	for i := range done {
		assert.True(t, portClosedAtCallback[i].Load())

		count++
		if count >= len(members) {
			close(done)
		}
	}
}

func newDummyCluster(t *testing.T, members []member, callbacks []func() error) ([]*Node, error) {
	logger, _ := zap.NewProduction()

	var nodes []*Node
	m := make(map[string]string)
	for _, member := range members {
		m[member.name] = fmt.Sprintf("https://127.0.0.1:%d", member.port)
	}

	for i, member := range members {
		tlsConfig, err := tlsutil.BuildTLSClientConfig(
			filepath.Join(baseTestPath, member.name, "peer", "tls.crt"),
			filepath.Join(baseTestPath, member.name, "peer", "tls.key"),
			[]string{filepath.Join(baseTestPath, "peer", "ca.crt")},
		)
		if err != nil {
			return nil, err
		}
		node, err := NewNode(logger, member.name, member.revision, m, tlsConfig, callbacks[i])
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}
