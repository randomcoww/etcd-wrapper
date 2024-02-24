package etcdutil

import (
	"context"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"io"
)

type MemberListResponseWithErr struct {
	*etcdserverpb.ResponseHeader
	Err error
}

type MemberAddResponseWithErr struct {
	*etcdserverpb.ResponseHeader
	*etcdserverpb.Member
	Err error
}

type MockNode struct {
	*etcdserverpb.StatusResponse
	*etcdserverpb.Member
	StatusErr error
}

type MockClient struct {
	NodeEndpointMap             map[string]*MockNode
	EndpointsResponse           []string
	SyncEndpointsErr            error
	HealthCheckErr              error
	DefragmentErr               error
	CreateSnapshotErr           error
	MemberAddResponseWithErr    *MemberAddResponseWithErr
	MemberListResponseWithErr   *MemberListResponseWithErr
	MemberRemoveResponseWithErr *MemberListResponseWithErr
}

func (m *MockClient) Close() error {
	return nil
}

func (m *MockClient) Endpoints() []string {
	return m.EndpointsResponse
}

func (m *MockClient) SyncEndpoints() error {
	return m.SyncEndpointsErr
}

func (m *MockClient) Status(handler func(Status, error)) {
	nodeSet := make(map[*MockNode]struct{})
	for _, endpoint := range m.EndpointsResponse {
		if node, ok := m.NodeEndpointMap[endpoint]; ok {
			if _, ok = nodeSet[node]; !ok {
				nodeSet[node] = struct{}{}
				handler(node.StatusResponse, node.StatusErr)
			}
		}
	}
}

func (m *MockClient) ListMembers() (List, error) {
	res := &etcdserverpb.MemberListResponse{
		Header:  m.MemberListResponseWithErr.ResponseHeader,
		Members: m.memberListResponse(),
	}
	return res, m.MemberListResponseWithErr.Err
}

func (m *MockClient) AddMember(peerURLs []string) (List, Member, error) {
	res := &etcdserverpb.MemberAddResponse{
		Header:  m.MemberAddResponseWithErr.ResponseHeader,
		Members: m.memberListResponse(),
	}
	return res, m.MemberAddResponseWithErr.Member, m.MemberAddResponseWithErr.Err
}

func (m *MockClient) RemoveMember(id uint64) (List, error) {
	res := &etcdserverpb.MemberRemoveResponse{
		Header:  m.MemberRemoveResponseWithErr.ResponseHeader,
		Members: m.memberListResponse(),
	}
	return res, m.MemberRemoveResponseWithErr.Err
}

func (m *MockClient) HealthCheck() error {
	m.memberListResponse()
	return m.HealthCheckErr
}

func (m *MockClient) Defragment(endpoint string) error {
	return m.DefragmentErr
}

func (m *MockClient) CreateSnapshot(handler func(context.Context, io.Reader) error) error {
	return m.CreateSnapshotErr
}

func (m *MockClient) memberListResponse() []*etcdserverpb.Member {
	var members []*etcdserverpb.Member
	var newEndpoints []string
	nodeSet := make(map[*MockNode]struct{})
	var foundEndpoint bool

	for endpoint, node := range m.NodeEndpointMap {
		if node.Member == nil {
			continue
		}
		newEndpoints = append(newEndpoints, endpoint)
		if _, ok := nodeSet[node]; !ok {
			nodeSet[node] = struct{}{}
			members = append(members, node.Member)
		}
		if !foundEndpoint && m.hasEndpoint(endpoint) {
			foundEndpoint = true
		}
	}

	if foundEndpoint {
		m.EndpointsResponse = newEndpoints
		return members
	}
	return []*etcdserverpb.Member{}
}

func (m *MockClient) hasEndpoint(checkEndpoint string) bool {
	for _, endpoint := range m.EndpointsResponse {
		if endpoint == checkEndpoint {
			return true
		}
	}
	return false
}

type MockClientResponses struct {
	Resp  []*MockClient
	Index int
}

func (r *MockClientResponses) InitialEndpoints() []string {
	return r.Resp[0].EndpointsResponse
}

func (r *MockClientResponses) Next(endpoints []string) *MockClient {
	client := r.Resp[r.Index]
	client.EndpointsResponse = endpoints
	r.Index++
	return client
}
