package wrapper

import (
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"strings"
)

type Member struct {
	id uint64
}

type MemberSet map[string]*Member

type MemberStatus struct {
	config    *config.Config
	localID   uint64
	memberSet MemberSet
}

func newMemberStatus(c *config.Config) *MemberStatus {
	s := &MemberStatus{
		config:    c,
		memberSet: MemberSet{},
	}

	for _, memberName := range s.config.MemberNames {
		s.memberSet[memberName] = &Member{}
	}
	return s
}

func (s *MemberStatus) mergeMemberList(memberList *clientv3.MemberListResponse) {
	memberFoundList := MemberSet{}
	var localID uint64
	var log []string

	for _, m := range memberList.Members {
		log = append(log, fmt.Sprintf("%v (%v)", m.ID, m.Name))

		// My ID matched by peerURL
		// It may not have a name yet if it was recently added
		if config.IsEqual(m.PeerURLs, s.config.LocalPeerURLs) {
			localID = m.ID
		}

		// New members may not have names yet
		if len(m.Name) == 0 {
			continue
		}

		if member, ok := s.memberSet[m.Name]; ok {
			member.id = m.ID
			memberFoundList[m.Name] = member
		} else {
			logrus.Warningf("[memberstatus] Removing unknown member: %v (%v)", m.Name, m.ID)
			s.removeMember(m.ID)
		}
	}
	s.localID = localID
	logrus.Infof("[memberstatus] Local: %v Members: %s", s.localID, strings.Join(log, ", "))

	// Go through members in config not returned by etcd and reset ID
	for memberName, member := range s.memberSet {
		if _, ok := memberFoundList[memberName]; !ok {
			member.id = 0
		}
	}
}

func (s *MemberStatus) addLocalMember() error {
	if s.localID != 0 {
		logrus.Warningf("[memberstatus] Local member ID already exists: %v", s.localID)
		return nil
	}

	resp, err := etcdutil.AddMember(s.config.ClientURLs, s.config.LocalPeerURLs, s.config.TLSConfig)
	switch err {
	case nil:
		s.localID = resp.Member.ID
		logrus.Infof("[memberstatus] Add member success: %v", resp.Member.ID)
		return nil
	default:
		logrus.Errorf("[memberstatus] Add member failed: %v", err)
		return err
	}
}

func (s *MemberStatus) removeLocalMember() error {
	return s.removeMember(s.localID)
}

func (s *MemberStatus) removeMember(memberID uint64) error {
	if memberID == 0 {
		logrus.Warningf("[memberstatus] Member ID not provided")
		return nil
	}

	err := etcdutil.RemoveMember(s.config.ClientURLs, s.config.TLSConfig, memberID)
	switch err {
	case nil, rpctypes.ErrMemberNotFound:
		s.localID = 0
		logrus.Infof("[memberstatus] Remove member success: %v", memberID)
		return nil
	default:
		logrus.Errorf("[memberstatus] Remove member failed (%v): %v", memberID, err)
		return err
	}
}
