package status

import (
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"gopkg.in/yaml.v3"
	"io"
	"sync"
)

type Member struct {
	*etcdserverpb.StatusResponse
	*etcdserverpb.Member
}

type Status struct {
	Healthy       bool                                  `yaml:"healthy"`
	ClusterID     uint64                                `yaml:"clusterID,omitempty"`
	Endpoints     []string                              `yaml:"endpoints,omitempty"`
	MemberMap     map[uint64]*Member                    `yaml:"members"`
	Self          *Member                               `yaml:"-"`
	Leader        *Member                               `yaml:"-"`
	mu            sync.Mutex                            `yaml:"-"`
	NewEtcdClient func([]string) (etcdutil.Util, error) `yaml:"-"`
}

// healthy if memberID from status matches ID returned from member list
func (m *Member) IsHealthy() bool {
	return m != nil && m.StatusResponse != nil && m.Member != nil &&
		m.StatusResponse.GetHeader().GetMemberId() == m.Member.GetID()
}

// check if this is a newly added member
func (m *Member) IsNew() bool {
	return m.Member != nil && (len(m.Member.GetName()) == 0 || len(m.Member.GetClientURLs()) == 0)
}

// Set initial client URLs
func New(args *arg.Args) *Status {
	status := &Status{
		Endpoints: args.AdvertiseClientURLs,
		NewEtcdClient: func(endpoints []string) (etcdutil.Util, error) {
			return etcdutil.New(endpoints, args.ClientTLSConfig)
		},
	}
	return status
}

func (v *Status) ToYaml() (b []byte, err error) {
	b, err = yaml.Marshal(v)
	return
}

func (v *Status) SyncStatus(args *arg.Args) error {
	v.Healthy = false
	v.ClusterID = 0
	v.Leader = nil
	v.Self = nil
	v.MemberMap = make(map[uint64]*Member)

	client, err := v.NewEtcdClient(v.Endpoints)
	if err != nil {
		return err
	}
	defer client.Close()

	err = client.HealthCheck()
	if err != nil {
		return nil
	}
	err = client.SyncEndpoints()
	if err != nil {
		return nil
	}
	v.Endpoints = client.Endpoints()

	list, err := client.ListMembers()
	if err != nil {
		return nil
	}
	v.Healthy = true
	v.UpdateFromList(list, args)

	// collect all members found by list and status
	client.Status(func(status etcdutil.Status, err error) {
		if err != nil {
			return
		}
		member := v.UpdateFromStatus(status, args)
		if !member.IsHealthy() {
			return
		}
		if v.Leader == nil {
			if member, ok := v.MemberMap[status.GetLeader()]; ok {
				v.Leader = member
			}
		}
	})

	err = v.SetSelf(args)
	if err != nil {
		return nil
	}
	return nil
}

func (v *Status) UpdateFromStatus(status etcdutil.Status, args *arg.Args) *Member {
	memberID := status.GetHeader().GetMemberId()
	if memberID == 0 {
		return nil
	}

	member, ok := v.MemberMap[memberID]
	if !ok {
		member = &Member{}
		v.MemberMap[memberID] = member
	}
	member.StatusResponse = status.(*etcdserverpb.StatusResponse)
	return member
}

func (v *Status) UpdateFromList(list etcdutil.List, args *arg.Args) {
	clusterID := list.GetHeader().GetClusterId()

	v.ClusterID = clusterID
	for _, m := range list.GetMembers() {
		memberID := m.GetID()
		if memberID == 0 {
			continue
		}

		member, ok := v.MemberMap[memberID]
		if !ok {
			member = &Member{}
			v.MemberMap[memberID] = member
		}
		member.Member = m
	}
}

// Find my node by sending a status check to advertiseClientURLs
func (v *Status) SetSelf(args *arg.Args) error {
	client, err := v.NewEtcdClient(args.AdvertiseClientURLs[:1])
	if err != nil {
		return err
	}
	defer client.Close()

	client.Status(func(status etcdutil.Status, err error) {
		if err != nil {
			return
		}
		v.Self = v.UpdateFromStatus(status, args)
	})
	return nil
}

// Find member from list that doesn't respond to status
// checkl if member has no status
func (v *Status) GetMemberToReplace() etcdutil.Member {
	for _, m := range v.MemberMap {
		if m.Member != nil && m.StatusResponse == nil && !m.IsNew() {
			return m.Member
		}
	}
	return nil
}

func (v *Status) ReplaceMember(m etcdutil.Member, args *arg.Args) error {
	client, err := v.NewEtcdClient(v.Endpoints)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.RemoveMember(m.GetID())
	if err != nil {
		return err
	}
	list, _, err := client.AddMember(m.GetPeerURLs())
	if err != nil {
		return err
	}
	err = client.SyncEndpoints()
	if err != nil {
		return nil
	}
	v.Endpoints = client.Endpoints()

	v.UpdateFromList(list, args)
	return nil
}

func (v *Status) Defragment(args *arg.Args) error {
	client, err := v.NewEtcdClient(v.Endpoints)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Defragment(args.AdvertiseClientURLs[0])
}

func (v *Status) SnapshotBackup(args *arg.Args) error {
	client, err := v.NewEtcdClient(v.Endpoints)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.CreateSnapshot(func(ctx context.Context, r io.Reader) error {
		return args.S3Client.Upload(ctx, args.S3BackupBucket, args.S3BackupKey, r)
	})
}
