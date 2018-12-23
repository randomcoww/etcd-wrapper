package cluster

import ()

type Reaper struct {
	Reset chan struct{}
	Fn    func(memberName string, memberID uint64)
}

type ReaperSet map[string]*Reaper
