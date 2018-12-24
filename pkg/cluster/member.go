package cluster

import (
)

type Member struct {
	Name string
	ID uint64
}

type MemberSet map[string]*Member

func NewMember(name string) *Member {
	return &Member{
		Name: name,
	}
}
