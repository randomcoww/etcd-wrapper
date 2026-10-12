package gossip

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var (
	reqTimeout       time.Duration = 2 * time.Second
	intervalDuration time.Duration = 1 * time.Second
)

type Peer struct {
	name       string
	url        string
	revision   int64
	client     *http.Client
	remoteDone atomic.Bool
}

type envelope struct {
	Name string           `json:"node"`
	View map[string]int64 `json:"view"`
	Done bool             `json:"done"`
}

type Node struct {
	logger    *zap.Logger
	name      string
	revision  int64
	peers     []*Peer
	tlsConfig *tls.Config
	listener  net.Listener
	server    *http.Server

	mu          sync.Mutex
	stopOnce    sync.Once
	manual      atomic.Bool
	fired       atomic.Bool
	onConverged func() error

	stop       chan struct{}
	serverDone chan struct{}
	loopDone   chan struct{}
}

func NewNode(logger *zap.Logger, name string, revision int64, peerURLsMap map[string]string, tlsConfig *tls.Config, handler func() error) (*Node, error) {
	n := &Node{
		logger:      logger,
		name:        name,
		revision:    revision,
		tlsConfig:   tlsConfig,
		onConverged: handler,
		stop:        make(chan struct{}),
		serverDone:  make(chan struct{}),
		loopDone:    make(chan struct{}),
	}

	for peerName, peerURL := range peerURLsMap {
		u, err := url.Parse(peerURL)
		if err != nil {
			return nil, fmt.Errorf("parse peerURL %s: %w", peerURL, err)
		}
		if name == peerName {
			listener, err := net.Listen("tcp", u.Host)
			if err != nil {
				return nil, fmt.Errorf("gossip listener %s: %w", peerURL, err)
			}
			n.listener = listener
			continue
		}
		n.peers = append(n.peers, &Peer{
			name: peerName,
			url:  fmt.Sprintf("%s://%s/gossip", u.Scheme, u.Host),
			client: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig:   tlsConfig,
					ForceAttemptHTTP2: false, // keep-alive h2 connections stall Shutdown
				},
				Timeout: reqTimeout,
			},
		})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /gossip", n.handleGossip)

	n.server = &http.Server{
		Handler:           mux,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 5 * time.Second,
		TLSNextProto:      map[string]func(*http.Server, *tls.Conn, http.Handler){},
	}
	return n, nil
}

func (n *Node) StartListen() {
	defer close(n.serverDone)
	n.logger.Info("start gossip listener", zap.String("node", n.name))

	if err := n.server.ServeTLS(n.listener, "", ""); err != nil &&
		!errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		n.logger.Error("server stopped with error", zap.Error(err))
	}
}

func (n *Node) StartLoop() error {
	defer close(n.loopDone)
	n.logger.Info("start gossip loop", zap.String("node", n.name))

	ticker := time.NewTicker(intervalDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
		case <-n.stop:
			n.logger.Info("gossip loop stopping")
			return nil
		}

		if n.converged() && n.verifyDone() {
			n.closeGossip()
			n.logger.Info("all nodes responded; gossip fully stopped")
			if !n.manual.Load() && n.fired.CompareAndSwap(false, true) {
				if err := n.onConverged(); err != nil {
					return err
				}
			}
			return nil
		}
		if err := n.round(); err != nil {
			n.logger.Error("round", zap.Error(err))
			return err
		}
	}
	return nil
}

func (n *Node) round() error {
	peer := n.peers[rand.IntN(len(n.peers))]
	reply, err := n.push(peer)
	if err != nil {
		if errors.Is(err, syscall.ECONNREFUSED) {
			peer.remoteDone.Store(true)
			return nil
		}
		return fmt.Errorf("gossip round failed: %w", err)
	}
	n.mergeEnvelope(reply)
	return nil
}

func (n *Node) verifyDone() bool {
	for _, peer := range n.peers {
		if peer.remoteDone.Load() {
			continue
		}

		reply, err := n.push(peer)
		if err != nil {
			switch {
			case errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, io.EOF):
				peer.remoteDone.Store(true)
				continue
			}
			n.logger.Error("gossip verify", zap.String("peer", peer.name), zap.Error(err))
			return false
		}
		n.logger.Info("gossip verify", zap.String("peer", peer.name), zap.String("envelope", fmt.Sprintf("%+v", reply)))

		if reply.Done {
			peer.remoteDone.Store(true)
		} else {
			return false
		}
	}
	n.logger.Info("gossip done", zap.String("node", n.name))
	return true
}

func (n *Node) closeGossip() {
	n.stopOnce.Do(func() {
		close(n.stop) // ends the gossip loop
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = n.server.Shutdown(ctx) // graceful; Shutdown also closes the listener
		_ = n.listener.Close()     // belt-and-braces: guarantee the port is gone
		<-n.serverDone
		n.logger.Info("gossip listener closed", zap.String("node", n.name))
	})
}

func (n *Node) push(peer *Peer) (envelope, error) {
	var reply envelope
	body, err := json.Marshal(n.buildEnvelope())
	if err != nil {
		return reply, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), intervalDuration)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, peer.url, bytes.NewReader(body))
	if err != nil {
		return reply, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := peer.client.Do(req)
	if err != nil {
		return reply, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return reply, fmt.Errorf("status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&reply); err != nil {
		return reply, err
	}
	n.logger.Info("gossip reply", zap.String("envelope", fmt.Sprintf("%+v", reply)), zap.Bool("done", reply.Done))
	return reply, nil
}

func (n *Node) handleGossip(w http.ResponseWriter, r *http.Request) {
	var env envelope
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&env); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	n.mergeEnvelope(env)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(n.buildEnvelope())
}

func (n *Node) mergeEnvelope(e envelope) {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, peer := range n.peers {
		if peer.name == e.Name && e.Done {
			peer.remoteDone.Store(true)
		}
		if rev, ok := e.View[peer.name]; ok {
			peer.revision = rev
		}
	}
}

func (n *Node) buildEnvelope() envelope {
	e := envelope{
		Name: n.name,
		View: make(map[string]int64),
		Done: true,
	}
	e.View[n.name] = n.revision

	n.mu.Lock()
	defer n.mu.Unlock()

	for _, peer := range n.peers {
		if peer.revision == 0 {
			e.Done = false
			continue
		}
		e.View[peer.name] = peer.revision
	}
	return e
}

func (n *Node) converged() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	for _, peer := range n.peers {
		if peer.revision == 0 {
			return false
		}
	}
	return true
}

func (n *Node) Stop() {
	n.manual.Store(true)
	n.closeGossip()
	select {
	case <-n.loopDone:
	case <-time.After(2 * time.Second):
	}
}
