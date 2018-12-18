package restore

import (
	"context"
	"fmt"
	"os"
	"io"
	"path/filepath"

	"github.com/coreos/etcd-operator/pkg/backup/reader"
)

type RestoreManager struct {
	rr reader.Reader
}

func NewRestoreManagerFromReader(rr reader.Reader) *RestoreManager {
	return &RestoreManager{
		rr: rr,
	}
}

func (rm *RestoreManager) DownloadSnap(ctx context.Context, s3Path, restorePath string) error {
	err := os.MkdirAll(filepath.Dir(restorePath), os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("failed to create base directory: (%v)", err)
	}

	f, err := os.OpenFile(restorePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open snaphot restore file: (%v)", err)
	}

	readCloser, err := rm.rr.Open(s3Path)
	if err != nil {
		return fmt.Errorf("failed to read backup: %v", err)
	}
	defer readCloser.Close()

	_, err = io.Copy(f, readCloser)
	if err != nil {
		return fmt.Errorf("failed to restore snapshot file: %v", err)
	}
	return err
}
