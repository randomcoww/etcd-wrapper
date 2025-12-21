package errors

import (
	"errors"
)

var (
	ErrNoCluster        = errors.New("controller: no cluster found from peers")
	ErrLocalNode        = errors.New("controller: could not get local node status")
	ErrCreateSnapshot   = errors.New("controller: failed to create snapshot")
	ErrUploadBackup     = errors.New("controller: failed to upload backup")
	ErrSnapshotEmpty    = errors.New("controller: smapshot file empty")
	ErrDefragment       = errors.New("controller: failed to run defragment")
	ErrDownloadSnapshot = errors.New("controller: failed to download snapshot")
	ErrRestoreSnapshot  = errors.New("controller: failed to load snapshot")
	ErrNoBackup         = errors.New("controller: no backup found")
)
