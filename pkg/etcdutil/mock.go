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
	Status    *etcdserverpb.StatusResponse
	StatusErr error
	Member    *etcdserverpb.Member
}

type MockClient struct {
	NodeEndpointMap              map[string]*MockNode
	EndpointsResponse            []string
	SyncEndpointsErr             error
	HealthCheckErr               error
	DefragmentErr                error
	CreateSnapshotErr            error
	MemberAddResponseWithErr     *MemberAddResponseWithErr
	MemberListResponseWithErr    *MemberListResponseWithErr
	MemberRemoveResponseWithErr  *MemberListResponseWithErr
	MemberPromoteResponseWithErr *MemberListResponseWithErr
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
	for _, resp := range m.getUniqueNodes(m.EndpointsResponse) {
		handler(resp.Status, resp.StatusErr)
	}
}

func (m *MockClient) ListMembers() (List, error) {
	res := &etcdserverpb.MemberListResponse{
		Header: m.MemberListResponseWithErr.ResponseHeader,
	}
	for _, resp := range m.getUniqueNodes(m.EndpointsResponse) {
		res.Members = append(res.Members, resp.Member)
		m.EndpointsResponse = append(m.EndpointsResponse, resp.Member.GetClientURLs()...)
	}
	return res, m.MemberListResponseWithErr.Err
}

func (m *MockClient) AddMember(peerURLs []string) (List, Member, error) {
	res := &etcdserverpb.MemberAddResponse{
		Header: m.MemberAddResponseWithErr.ResponseHeader,
	}
	for _, resp := range m.getUniqueNodes(m.EndpointsResponse) {
		res.Members = append(res.Members, resp.Member)
		m.EndpointsResponse = append(m.EndpointsResponse, resp.Member.GetClientURLs()...)
	}
	return res, m.MemberAddResponseWithErr.Member, m.MemberAddResponseWithErr.Err
}

func (m *MockClient) RemoveMember(id uint64) (List, error) {
	res := &etcdserverpb.MemberRemoveResponse{
		Header: m.MemberRemoveResponseWithErr.ResponseHeader,
	}
	for _, resp := range m.getUniqueNodes(m.EndpointsResponse) {
		if resp.Member.GetID() != id {
			res.Members = append(res.Members, resp.Member)
			m.EndpointsResponse = append(m.EndpointsResponse, resp.Member.GetClientURLs()...)
		}
	}
	return res, m.MemberRemoveResponseWithErr.Err
}

func (m *MockClient) PromoteMember(id uint64) (List, error) {
	res := &etcdserverpb.MemberPromoteResponse{
		Header: m.MemberPromoteResponseWithErr.ResponseHeader,
	}
	for _, resp := range m.getUniqueNodes(m.EndpointsResponse) {
		res.Members = append(res.Members, resp.Member)
		m.EndpointsResponse = append(m.EndpointsResponse, resp.Member.GetClientURLs()...)
	}
	return res, m.MemberPromoteResponseWithErr.Err
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

func (m *MockClient) getUniqueNodes(endpoints []string) []*MockNode {
	var nodes []*MockNode
	uniqueNodes := make(map[*MockNode]struct{})
	for _, endpoint := range endpoints {
		nodeResp := m.NodeEndpointMap[endpoint]
		if _, ok := uniqueNodes[nodeResp]; nodeResp != nil && !ok {
			uniqueNodes[nodeResp] = struct{}{}
			nodes = append(nodes, nodeResp)
		}
	}
	return nodes
}
