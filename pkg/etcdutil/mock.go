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
	*etcdserverpb.ResponseHeader
	Members []*etcdserverpb.Member
	Err     error
}

type MemberAddResponseWithErr struct {
	*etcdserverpb.ResponseHeader
	*etcdserverpb.Member
	Members []*etcdserverpb.Member
	Err     error
}

type MockClient struct {
	EndpointsResponse           []string
	SyncEndpointsErr            error
	HealthCheckErr              error
	DefragmentErr               error
	CreateSnapshotErr           error
	StatusResponseWithErr       map[string]*StatusResponseWithErr
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
	if m.MemberListResponseWithErr.Err != nil {
		return m.MemberListResponseWithErr.Err
	}
	m.EndpointsResponse = []string{}
	for _, member := range m.MemberListResponseWithErr.Members {
		m.EndpointsResponse = append(m.EndpointsResponse, member.ClientURLs...)
	}
	return nil
}

func (m *MockClient) Status(handler func(Status, error)) {
	for _, endpoint := range m.Endpoints() {
		if resp, ok := m.StatusResponseWithErr[endpoint]; ok {
			handler(resp.StatusResponse, resp.Err)
		}
	}
}

func (m *MockClient) ListMembers() (List, error) {
	return &etcdserverpb.MemberListResponse{
		Header:  m.MemberListResponseWithErr.ResponseHeader,
		Members: m.MemberListResponseWithErr.Members,
	}, m.MemberListResponseWithErr.Err
}

func (m *MockClient) AddMember(peerURLs []string) (List, Member, error) {
	return &etcdserverpb.MemberAddResponse{
		Header:  m.MemberAddResponseWithErr.ResponseHeader,
		Members: m.MemberAddResponseWithErr.Members,
	}, m.MemberAddResponseWithErr.Member, m.MemberAddResponseWithErr.Err
}

func (m *MockClient) RemoveMember(id uint64) (List, error) {
	return &etcdserverpb.MemberRemoveResponse{
		Header:  m.MemberRemoveResponseWithErr.ResponseHeader,
		Members: m.MemberRemoveResponseWithErr.Members,
	}, m.MemberRemoveResponseWithErr.Err
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

// type MockClientResponses struct {
// 	Resp  []*MockClient
// 	Index int
// }

// func (r *MockClientResponses) InitialEndpoints() []string {
// 	return r.Resp[0].EndpointsResponse
// }

// func (r *MockClientResponses) Next(endpoints []string) *MockClient {
// 	client := r.Resp[r.Index]
// 	client.EndpointsResponse = endpoints
// 	if r.Index < len(r.Resp)-1 {
// 		r.Index++
// 	}
// 	return client
// }
