package etcdutil

import (
	"context"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"io"
)

type StatusResponseWithErr struct {
	*etcdserverpb.StatusResponse
	Err error
}

type MemberListResponseWithErr struct {
	Header *etcdserverpb.ResponseHeader
	Err    error
}

type MemberAddResponseWithErr struct {
	Header *etcdserverpb.ResponseHeader
	Member *etcdserverpb.Member
	Err    error
}

type MockNode struct {
	MemberResponse        Member
	StatusResponseWithErr *StatusResponseWithErr
}

type MockClient struct {
	NodeEndpointMap              map[string]*MockNode
	EndpointsResponse            []string
	SyncEndpointsErr             error
	MemberAddResponseWithErr     *MemberAddResponseWithErr
	MemberListResponseWithErr    *MemberListResponseWithErr
	MemberRemoveResponseWithErr  *MemberListResponseWithErr
	MemberPromotResponseWitheErr *MemberListResponseWithErr
	HealthCheckErr               error
	DefragmentErr                error
	CreateSnapshotErr            error
	endpoints                    []string
}

func (m *MockClient) Close() error {
	return nil
}

func (m *MockClient) Sync(ctx context.Context) error {
	m.EndpointsResponse = m.endpoints
	return nil
}

func (m *MockClient) Endpoints() []string {
	return m.EndpointsResponse
}

func (m *MockClient) SyncEndpoints() error {
	if m.SyncEndpointsErr != nil {
		return m.SyncEndpointsErr
	}
	m.Sync(context.Background())
	return nil
}

func (m *MockClient) Status(handler func(Status, error)) {
	nodeResps := m.getUniqueNodes(m.Endpoints())
	for _, resp := range nodeResps {
		handler(resp.StatusResponseWithErr.StatusResponse, resp.StatusResponseWithErr.Err)
	}
}

func (m *MockClient) ListMembers() (List, error) {
	var membersResp []*etcdserverpb.Member
	nodeResps := m.getUniqueNodes(m.Endpoints())
	for _, resp := range nodeResps {
		membersResp = append(membersResp, resp.Member)
		m.endpoints = append(m.endpoints, resp.GetClientURLs()...)
	}
	return &etcdserverpb.MemberListResponse{
		Header:  MemberListResponseWithErr.Header,
		Members: membersResp,
	}, m.MemberListResponseWithErr.Err
}

func (m *MockClient) AddMember(peerURLs []string) (List, Member, error) {
	var membersResp []*etcdserverpb.Member
	nodeResps := m.getUniqueNodes(m.Endpoints())
	for _, resp := range nodeResps {
		membersResp = append(membersResp, resp.Member)
		m.endpoints = append(m.endpoints, resp.GetClientURLs()...)
	}
	return &etcdserverpb.MemberAddResponse{
		Header:  MemberAddResponseWithErr.Header,
		Members: membersResp,
	}, m.MemberAddResponseWithErr.Member, m.MemberAddResponseWithErr.Err
}

func (m *MockClient) RemoveMember(id uint64) (List, error) {
	var membersResp []Member
	nodeResps := m.getUniqueNodes(m.Endpoints())
	for _, resp := range nodeResps {
		if resp.Member.GetID() == id {
			continue
		}
		membersResp = append(membersResp, resp.Member)
		m.endpoints = append(m.endpoints, resp.GetClientURLs()...)
	}
	return &etcdserverpb.MemberRemoveResponse{
		Header:  MemberRemoveResponseWithErr.Header,
		Members: membersResp,
	}, MemberRemoveResponseWithErr.Err
}

func (m *MockClient) PromoteMember(id uint64) (List, error) {
	var membersResp []*etcdserverpb.Member
	nodeResps := m.getUniqueNodes(m.Endpoints())
	for _, resp := range nodeResps {
		membersResp = append(membersResp, resp.Member)
		m.endpoints = append(m.endpoints, resp.GetClientURLs()...)
	}
	return &etcdserverpb.MemberPromoteResponse{
		Header:  MemberPromoteResponseWithErr.Header,
		Members: membersResp,
	}, MemberPromoteResponseWithErr.Err
}

func (m *MockClient) HealthCheck() error {
	return m.HealthCheckErr
}

func (m *MockClient) Defragment(endpoint string) error {
	return m.DefragmentErr
}

func (m *MockClient) CreateSnapshot(handler func(context.Context, io.Reader) error) error {
	return m.CreateSnapshotErr
}

func (m *MockClient) getUniqueNodes(endpoints []string) []*Node {
	var nodes []*Node
	uniqueNodes := make(map[*Node]struct{})
	for _, endpoint := range endpoints {
		nodeResp := m.NodeEndpointMap[endpoint]
		if _, ok := uniqueNodes[*Node]; nodeResp != nil && !ok {
			uniqueNodes[nodeResp] = struct{}{}
			nodes = append(nodes, nodeResp)
		}
	}
	return nodes
}
