func (v *status) UpdateFromList() error {
	var clients []string
	for _, member := range v.MemberMap {
		clients = append(clients, member.ClientURL)
	}

	members, err := v.ListMembers(clients)
	if err != nil {
		return err
	}

	// member Name field may not be populated right away
	// Match returned members by PeerURL field
	peerURLsReturned := make(map[string]struct{})
	for _, member := range members.Members {
		var m *Member
		var ok bool

		for _, peer := range member.PeerURLs {
			var id uint64
			if m, ok = v.MemberPeerMap[peer]; ok {
				memberID = member.ID
				m.MemberID = &memberID

				peerURLsReturned[peer] = struct{}{}
				break
			}
		}
	}

	// Compare returned members with list and remove inactive ones
	for peer, m := range v.MemberPeerMap {
		if _, ok := peerURLsReturned[peer]; !ok {
			m.MemberID = nil
		}
	}
}

func (v *Status) UpdateFromStatus() error {
	var clients []string
	for _, member := range v.MemberMap {
		clients = append(clients, member.ClientURL)
	}

	respCh, err := v.GetStatus(clients, v.ClientTLSConfig)
	if err != nil {
		return err
	}

	for i, resp := range <-respCh {
		m, ok := v.MemberClientMap[resp.endpoint]
		if !ok || m == nil {
			continue
		}
		if resp.err != nil {
			m.MemberID = nil
			m.ClusterID = nil
			m.Revision = nil
			m.IsLeader = false
			continue
		}

		memberID := resp.status.ResponseHeader.MemberID
		clusterID := resp.status.ResponseHeader.ClusterID
		revision := resp.status.ResponseHeader.Revision

		if v.ClusterID == nil {
			v.ClusterID = &clusterID
		}
		if *v.ClusterID != clusterID {
			m.ClusterID = nil
			m.IsLeader = false
			continue
		}
		if v.Revision == nil {
			v.Revision = &revision
		}
		if *v.Revision < revision {
			v.Revision = &revision
		}

		m.MemberID = &memberID
		m.ClusterID = &clusterID
		m.Revision = &revision
		m.IsLeader = resp.status.Leader == resp.status.ResponseHeader.MemberID
	}
}



// func (v *status) GetMaxRevisionMember() (*Member, error) {
// 	var maxRevision uint64
// 	var member *Member
// 	err := v.UpdateFromStatus()
// 	if err != nil {
// 		return nil, err
// 	}
// 	for _, m := range v.MemberNameMap {
// 		if member == nil {
// 			maxRevision = m.Revision
// 		}
// 		if maxRevision < m.Revision {
// 			maxRevision = m.Revision
// 			member = m
// 		}
// 	}
// 	return m, nil
// }

// func (v *status) CheckSplitBrain() bool {
// 	clusterMemberCounts := make(map[*uint64]int)
// 	var maxClusterSize int

// 	for _, m := range v.MemberNameMap {
// 		clusterMemberCounts[m.ClusterID]++
// 		if clusterMemberCounts[m.ClusterID] > maxClusterSize {
// 			maxClusterSize = clusterMemberCounts[m.ClusterID]
// 		}
// 	}
// 	// members have different clusterIDs - split brain?
// 	if len(clusterMemberCounts) > 1 {
// 		if clusterIDCounts[v.MemberSelf.ClusterID] <= maxCount {
// 			return true
// 		}
// 	}
// 	return false
// }

func (v *status) HasSplitBrain() bool {
	clusterMemberCounts := make(map[*uint64]int)
	var maxClusterSize int

	for _, m := range v.MemberNameMap {
		clusterMemberCounts[m.ClusterID]++
		if clusterMemberCounts[m.ClusterID] > maxClusterSize {
			maxClusterSize = clusterMemberCounts[m.ClusterID]
		}
	}
	// members have different clusterIDs - split brain?
	if len(clusterMemberCounts) > 1 {
		if clusterIDCounts[v.MemberSelf.ClusterID] <= maxCount {
			return true
		}
	}
	return false
}

func (v *status) HasSplitBrain() bool {
	clusterMemberCounts := make(map[*uint64]int)
	var maxClusterSize int

	for _, m := range v.MemberNameMap {
		clusterMemberCounts[m.ClusterID]++
		if clusterMemberCounts[m.ClusterID] > maxClusterSize {
			maxClusterSize = clusterMemberCounts[m.ClusterID]
		}
	}
	// members have different clusterIDs - split brain?
	if len(clusterMemberCounts) > 1 {
		if clusterIDCounts[v.MemberSelf.ClusterID] <= maxCount {
			return true
		}
	}
	return false
}
