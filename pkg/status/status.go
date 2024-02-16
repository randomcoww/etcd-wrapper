package status

import (
	"context"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	"gopkg.in/yaml.v3"
	"io"
	"sync"
)

type Member struct {
	Status etcdutil.Status
	Member etcdutil.Member
}

type Status struct {
	Healthy       bool               `yaml:"healthy"`
	ClusterID     uint64             `yaml:"clusterID,omitempty"`
	Endpoints     []string           `yaml:"endpoints,omitempty"`
	MemberMap     map[uint64]*Member `yaml:"members"`
	Self          *Member            `yaml:"-"`
	Leader        *Member            `yaml:"-"`
	mu            sync.Mutex
	NewEtcdClient func([]string) (etcdutil.Util, error)
}

func (m *Member) Healthy() bool {
	return m.Status != nil && m.Member != nil &&
		m.Status.GetHeader().GetMemberId() == m.Member.GetID()
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
	v.Healthy = true

	list, err := client.ListMembers()
	if err != nil {
		return nil
	}
	v.UpdateFromList(list)

	// collect all members found by list and status
	client.Status(func(status etcdutil.Status, err error) {
		if err != nil {
			return
		}
		member := v.UpdateFromStatus(status)
		if member == nil {
			return
		}
		if !member.Healthy() {
			return
		}

		if status.GetLeader() != 0 {
			if leader, ok := v.MemberMap[status.GetLeader()]; ok {
				v.Leader = leader
			}
		}

		// check if member clientURLs match one of my clientURLs and assign self
	L:
		for _, sent := range args.AdvertiseClientURLs {
			for _, recv := range member.Member.GetClientURLs() {
				if sent == recv {
					v.Self = member
					break L
				}
			}
		}
	})
	return nil
}

func (v *Status) UpdateFromStatus(status etcdutil.Status) *Member {
	memberID := status.GetHeader().GetMemberId()
	if memberID == 0 {
		return nil
	}

	member, ok := v.MemberMap[memberID]
	if !ok {
		member = &Member{}
		v.MemberMap[memberID] = member
	}
	member.Status = status
	return member
}

func (v *Status) UpdateFromList(list etcdutil.List) {
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

func (v *Status) ReplaceMember(args *arg.Args) error {
	client, err := v.NewEtcdClient(v.Endpoints)
	if err != nil {
		return err
	}
	defer client.Close()

	_, err = client.RemoveMember(v.Self.Member.GetID())
	if err != nil {
		return err
	}
	list, _, err := client.AddMember(args.ListenPeerURLs)
	if err != nil {
		return err
	}
	err = client.SyncEndpoints()
	if err != nil {
		return nil
	}

	v.Endpoints = client.Endpoints()

	v.UpdateFromList(list)
	return nil
}

func (v *Status) PromoteMember(args *arg.Args) error {
	client, err := v.NewEtcdClient(v.Endpoints)
	if err != nil {
		return err
	}
	defer client.Close()

	list, err := client.PromoteMember(v.Self.Member.GetID())
	if err != nil {
		return err
	}
	err = client.SyncEndpoints()
	if err != nil {
		return nil
	}
	v.Endpoints = client.Endpoints()

	v.UpdateFromList(list)
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
