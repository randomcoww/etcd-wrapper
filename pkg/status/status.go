package status

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/podspec"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"gopkg.in/yaml.v3"
	"io"
	"k8s.io/api/core/v1"
	"log"
	"time"
)

type Member struct {
	Name                string  `yaml:"-"`
	Healthy             bool    `yaml:"healthy"`
	MemberID            *uint64 `yaml:"memberID,omitempty"`
	MemberIDFromCluster *uint64 `yaml:"memberIDFromCluster,omitempty"`
	Revision            *int64  `yaml:"revision,omitempty"`
	ClusterID           *uint64 `yaml:"clusterID,omitempty"`
	LeaderID            *uint64 `yaml:"leaderID,omitempty"`
	Self                bool    `yaml:"self,omitempty"`
	//
	PeerURL   string `yaml:"-"`
	ClientURL string `yaml:"-"`
}

type Status struct {
	Healthy        bool    `yaml:"healthy"`
	ClusterID      *uint64 `yaml:"clusterID,omitempty"`
	LeaderID       *uint64 `yaml:"leaderID,omitempty"`
	BackupMemberID *uint64 `yaml:"backupMemberID,omitempty"`
	Revision       *int64  `yaml:"revision,omitempty"`
	//
	MemberMap          map[string]*Member `yaml:"members"`
	MemberPeerURLMap   map[string]*Member `yaml:"-"`
	MemberClientURLMap map[string]*Member `yaml:"-"`
	MemberSelf         *Member            `yaml:"-"`
	Members            []*Member          `yaml:"-"`
	MembersHealthy     []*Member          `yaml:"-"`
	//
	clientURLs []string
}

func New(args *arg.Args) (*Status, error) {
	v := &Status{}
	v.MemberMap = make(map[string]*Member)
	v.MemberPeerURLMap = make(map[string]*Member)
	v.MemberClientURLMap = make(map[string]*Member)

	for _, node := range args.InitialCluster {
		member := &Member{
			Name:      node.Name,
			ClientURL: node.ClientURL,
			PeerURL:   node.PeerURL,
		}
		v.Members = append(v.Members, member)
		v.MemberMap[member.Name] = member
		v.MemberPeerURLMap[member.PeerURL] = member
		v.MemberClientURLMap[member.ClientURL] = member
		if member.Name == args.Name {
			member.Self = true
			v.MemberSelf = member
		}
		v.clientURLs = append(v.clientURLs, member.ClientURL)
	}
	return v, nil
}

func (v *Status) ToYaml() (b []byte, err error) {
	b, err = yaml.Marshal(v)
	return
}

func (v *Status) UpdateFromStatus(args *arg.Args, etcd etcdutil.StatusCheck) error {
	clusterIDCount := make(map[uint64]int)

	err := etcd.Status(v.clientURLs, func(status *etcdutil.StatusResp, err error) {
		// endpoint will be returned even if there is an error
		m, ok := v.MemberClientURLMap[status.Endpoint]
		if !ok {
			return
		}
		m.ClientURL = status.Endpoint

		if err != nil {
			m.ClusterID = nil
			m.LeaderID = nil
			m.Revision = nil
			m.MemberID = nil
			return
		}

		m.ClusterID = status.ClusterID
		m.LeaderID = status.LeaderID
		m.Revision = status.Revision
		m.MemberID = status.MemberID

		// set cluster wide IDs by majority found among members
		// 1. cluster ID nil
		// 2. more members belonging to cluster ID
		// 3. same member count in multiple IDs, but ID value is smaller
		clusterIDCount[*status.ClusterID]++
		if v.ClusterID == nil ||
			clusterIDCount[*status.ClusterID] > clusterIDCount[*v.ClusterID] ||
			(clusterIDCount[*status.ClusterID] == clusterIDCount[*v.ClusterID] && *status.ClusterID < *v.ClusterID) {
			v.ClusterID = status.ClusterID
			v.LeaderID = status.LeaderID
			if v.Revision == nil || *v.Revision < *status.Revision {
				v.Revision = status.Revision
			}
		}
	}, args.ClientTLSConfig)
	if err != nil {
		v.Healthy = false
		return err
	}

	// filter out members in wrong cluster ID
	v.clientURLs = []string{}
	for _, m := range v.Members {
		if *m.ClusterID != *v.ClusterID {
			v.clientURLs = append(v.clientURLs, m.ClientURL)
		}
	}

	// check list from members in matching cluster ID
	members, err := etcd.ListMembers(v.clientURLs, args.ClientTLSConfig)
	if err != nil {
		v.Healthy = false
		return err
	}

	// update client URLs again from list to include newly joined members
	// assume memberID reported by self and memberID reported by majority matching means in cluster
	v.clientURLs = []string{}
	for _, resp := range members {
		// find member by one of the peerURLs
		for _, peerURL := range resp.PeerURLs {
			if m, ok := v.MemberPeerURLMap[peerURL]; ok {
				m.MemberIDFromCluster = resp.ID
				v.clientURLs = append(v.clientURLs, m.ClientURL)
				break
			}
		}
	}

	// check cluster health
	err = etcd.HealthCheck(v.clientURLs, args.ClientTLSConfig)
	if err != nil {
		v.Healthy = false
		return err
	}
	v.Healthy = true
	return nil
}

func (v *Status) SetMembersHealth() {
	for _, m := range v.Members {
		switch {
		case !v.Healthy, m.MemberID == nil, m.MemberIDFromCluster == nil, m.ClusterID == nil:
			m.Healthy = false
		case *m.ClusterID != *v.ClusterID:
			m.Healthy = false
		case *m.MemberIDFromCluster != *m.MemberID:
			m.Healthy = false
		default:
			m.Healthy = true
		}
	}
}

func (v *Status) GetBackupMember() *Member {
	if v.Healthy {
		var member *Member
		for _, m := range v.Members {
			switch {
			case !m.Healthy:
				continue
			case *m.Revision < *v.Revision:
				continue
			case member == nil, *m.MemberID < *member.MemberID:
				member = m
			}
		}
		return member
	}
	return nil
}

func (v *Status) ReplaceMember(args *arg.Args, m *Member) error {
	if m.MemberIDFromCluster != nil {
		err := etcdutil.RemoveMember(v.clientURLs, args.ClientTLSConfig, *m.MemberIDFromCluster)
		if err != nil {
			return err
		}
		log.Printf("Removed member %v", *m.MemberIDFromCluster)
	}
	member, err := etcdutil.AddMember(v.clientURLs, args.ListenPeerURLs, args.ClientTLSConfig)
	if err != nil {
		return err
	}
	m.MemberID = member.ID
	m.MemberIDFromCluster = member.ID

	v.SetMembersHealth()
	return nil
}

func (v *Status) WritePodManifest(args *arg.Args, runRestore bool) error {
	var pod *v1.Pod
	manifestVersion := fmt.Sprintf("%v", time.Now().Unix())
	if runRestore {
		ok, err := v.RestoreSnapshot(args)
		if err != nil {
			return fmt.Errorf("Error getting snapshot: %v", err)
		}
		if !ok {
			log.Printf("Snapshot not found. Starting new cluster")
			args.InitialClusterState = "new"
			pod = podspec.Create(args, false, manifestVersion)

		} else {
			log.Printf("Successfully got snapshot. Restoring cluster")
			args.InitialClusterState = "existing"
			pod = podspec.Create(args, true, manifestVersion)
		}
	} else {
		args.InitialClusterState = "existing"
		pod = podspec.Create(args, false, manifestVersion)
	}

	manifest, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteFile(io.NopCloser(bytes.NewReader(manifest)), args.EtcdPodManifestFile)
}

func (v *Status) DeletePodManifest(args *arg.Args) error {
	return util.DeleteFile(args.EtcdPodManifestFile)
}

func (v *Status) Defragment(args *arg.Args) error {
	if v.Healthy {
		err := etcdutil.Defragment(v.MemberSelf.ClientURL, args.ClientTLSConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Status) BackupSnapshot(args *arg.Args) error {
	m := v.GetBackupMember()
	if m == v.MemberSelf {
		return etcdutil.CreateSnapshot(v.clientURLs, args.ClientTLSConfig, func(ctx context.Context, r io.Reader) error {
			return args.S3Client.Upload(ctx, args.S3BackupBucket, args.S3BackupKey, r)
		})
	}
	return nil
}

func (v *Status) RestoreSnapshot(args *arg.Args) (bool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return args.S3Client.Download(ctx, args.S3BackupBucket, args.S3BackupKey, func(ctx context.Context, r io.Reader) error {
		return util.WriteFile(r, args.EtcdSnapshotFile)
	})
}
