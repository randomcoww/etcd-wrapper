package etcdutil

import (
	"context"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"io"
)

type MockList struct {
	Header  *MockHeader
	Members []*MockMember
	Member  *MockMember
	Err     error
}

type MockStatus struct {
	Header    *MockHeader
	Leader    uint64
	RaftIndex uint64
	IsLearner bool
	Err       error
}

type MockHeader struct {
	ClusterId uint64
	MemberId  uint64
	Revision  int64
}

type MockMember struct {
	ID         uint64
	ClientURLs []string
	PeerURLs   []string
	IsLearner  bool
}

// list

func (m *MockList) GetHeader() *etcdserverpb.ResponseHeader {
	return m.Header.resp()
}

func (m *MockList) GetMembers() []*etcdserverpb.Member {
	var resp []*etcdserverpb.Member
	for _, m := range m.Members {
		resp = append(resp, m.resp())
	}
	return resp
}

// status

func (m *MockStatus) GetHeader() *etcdserverpb.ResponseHeader {
	return m.Header.resp()
}

func (m *MockStatus) GetIsLearner() bool {
	return m.IsLearner
}

func (m *MockStatus) GetLeader() uint64 {
	return m.Leader
}

func (m *MockStatus) GetRaftIndex() uint64 {
	return m.RaftIndex
}

// header

func (m *MockHeader) GetClusterId() uint64 {
	return m.ClusterId
}

func (m *MockHeader) GetMemberId() uint64 {
	return m.MemberId
}

func (m *MockHeader) GetRevision() int64 {
	return m.Revision
}

func (m *MockHeader) resp() *etcdserverpb.ResponseHeader {
	return &etcdserverpb.ResponseHeader{
		ClusterId: m.GetClusterId(),
		MemberId:  m.GetMemberId(),
		Revision:  m.GetRevision(),
	}
}

// member

func (m *MockMember) GetID() uint64 {
	return m.ID
}

func (m *MockMember) GetClientURLs() []string {
	return m.ClientURLs
}

func (m *MockMember) GetPeerURLs() []string {
	return m.PeerURLs
}

func (m *MockMember) GetIsLearner() bool {
	return m.IsLearner
}

func (m *MockMember) resp() *etcdserverpb.Member {
	return &etcdserverpb.Member{
		ID:         m.GetID(),
		PeerURLs:   m.GetPeerURLs(),
		ClientURLs: m.GetClientURLs(),
		IsLearner:  m.GetIsLearner(),
	}
}

// client

type MockClient struct {
	EndpointsResponse     []string
	SyncEndpointsErr      error
	StatusResponses       []*MockStatus
	ListMembersReponse    *MockList
	AddMemberResponse     *MockList
	RemoveMemberResponse  *MockList
	PromoteMemberResponse *MockList
	HealthCheckErr        error
	DefragmentErr         error
	CreateSnapshotErr     error
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
		handler(resp, resp.Err)
	}
}

func (m *MockClient) ListMembers() (List, error) {
	return m.ListMembersReponse, m.ListMembersReponse.Err
}

func (m *MockClient) AddMember(peerURLs []string) (List, Member, error) {
	return m.AddMemberResponse, m.AddMemberResponse.Member, m.AddMemberResponse.Err
}

func (m *MockClient) RemoveMember(id uint64) (List, error) {
	return m.RemoveMemberResponse, m.RemoveMemberResponse.Err
}

func (m *MockClient) PromoteMember(id uint64) (List, error) {
	return m.PromoteMemberResponse, m.PromoteMemberResponse.Err
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
