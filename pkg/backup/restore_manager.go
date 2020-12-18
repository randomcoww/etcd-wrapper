package backup

import (
	"context"
	"fmt"
	"io"
	"os"
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

func (rm *RestoreManager) DownloadSnap(ctx context.Context, source, restorePath string) error {
	err := os.MkdirAll(filepath.Dir(restorePath), os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("failed to create base directory %s: %v", restorePath, err)
	}

	f, err := os.OpenFile(restorePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %v", restorePath, err)
	}
	defer f.Close()

	readCloser, err := rm.rr.Open(source)
	if err != nil {
		return fmt.Errorf("failed to read backup source: %v", err)
	}
	defer readCloser.Close()

	_, err = io.Copy(f, readCloser)
	if err != nil {
		return fmt.Errorf("failed to restore download backup: %v", err)
	}
	return err
}
