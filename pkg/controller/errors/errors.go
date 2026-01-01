package errors

import (
	"errors"
)

var (
	ErrSnapshotDir      = errors.New("controller: failed to create snapshot file for restore")
	ErrDownloadSnapshot = errors.New("controller: failed to download snapshot")
	ErrRestoreSnapshot  = errors.New("controller: failed to restore snapshot")
	ErrNoBackup         = errors.New("controller: no backup found")
	ErrDataDir          = errors.New("controller: error accessing data dir")
	ErrMemberList       = errors.New("controller: error runnning etcd member list")
	ErrMemberAdd        = errors.New("controller: error runnning etcd member add")
	ErrMemberRemove     = errors.New("controller: error runnning etcd member remove")
)
