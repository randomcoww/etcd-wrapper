package snapshot

import (
	"context"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/s3util"
	"io"
	"os"
	"path/filepath"
)

func (v *Status) RestoreSnapshot(readCloser io.ReadCloser, restoreFile string) error {
	defer readCloser.Close()

	err := os.MkdirAll(filepath.Dir(restoreFile), os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("failed to create base directory %s: %v", restoreFile, err)
	}

	f, err := os.OpenFile(restoreFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", restoreFile, err)
	}
	defer f.Close()

	_, err = io.Copy(f, readCloser)
	if err != nil {
		return fmt.Errorf("failed to restore download backup: %v", err)
	}
	return err
}

func (v *Status) BackupSnapshot(s3resource string, writer io.Writer) error {
	err := v.SyncStatus()
	if err != nil {
		return err
	}
	err := v.PickBackupMember()
	if err != nil {
		return err
	}
	if v.MemberSelf.MemberID != v.BackupMemberID {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	rc, err := etcdcli.Snapshot(ctx)
	if err != nil {
		return err
	}
	defer rc.Close()

	writer := s3Util.NewWriter()
	writer.Write(ctx, s3Resource, rc)
	cancel()

	return nil
}