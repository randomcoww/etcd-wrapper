package etcdutil

import (
	"context"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"io"
)

type MockClient struct {
	EndpointsResponse []string
	SyncEndpointsErr  error
	StatusResponses   []struct {
		*etcdserverpb.StatusResponse
		Err error
	}
	MemberListResponse struct {
		*etcdserverpb.MemberListResponse
		Err error
	}
	MemberAddResponse struct {
		*etcdserverpb.MemberAddResponse
		Err error
	}
	MemberRemoveResponse struct {
		*etcdserverpb.MemberRemoveResponse
		Err error
	}
	MemberPromoteResponse struct {
		*etcdserverpb.MemberPromoteResponse
		Err error
	}
	HealthCheckErr    error
	DefragmentErr     error
	CreateSnapshotErr error
}

func (m *MockClient) Close() error {
	return nil
}

func (m *MockClient) Sync(ctx context.Context) error {
	return nil
}

func (m *MockClient) Endpoints() []string {
	return m.EndpointsResponse
}

func (m *MockClient) SyncEndpoints() error {
	return m.SyncEndpointsErr
}

func (m *MockClient) Status(handler func(Status, error)) {
	for _, resp := range m.StatusResponses {
		handler(resp.StatusResponse, resp.Err)
	}
}

func (m *MockClient) ListMembers() (List, error) {
	return m.MemberListResponse.MemberListResponse, m.MemberListResponse.Err
}

func (m *MockClient) AddMember(peerURLs []string) (List, Member, error) {
	return m.MemberAddResponse.MemberAddResponse, m.MemberAddResponse.GetMember(), m.MemberAddResponse.Err
}

func (m *MockClient) RemoveMember(id uint64) (List, error) {
	return m.MemberRemoveResponse.MemberRemoveResponse, m.MemberRemoveResponse.Err
}

func (m *MockClient) PromoteMember(id uint64) (List, error) {
	return m.MemberPromoteResponse.MemberPromoteResponse, m.MemberPromoteResponse.Err
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
