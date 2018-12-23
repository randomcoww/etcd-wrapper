package cluster

import ()

type Reaper struct {
	Reset chan struct{}
	Fn    func(memberName string, memberID uint64, isMyNode bool)
}

type ReaperSet map[string]*Reaper

func (r *Reaper) ResetTimeout() {
	select {
	case r.Reset <- struct{}{}:
	default:
	}
}
