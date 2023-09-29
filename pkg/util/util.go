package util

import (
	"io"
	"os"
	"path/filepath"
)

func WriteFile(rc io.Reader, writePath string) error {
	err := os.MkdirAll(filepath.Dir(writePath), os.FileMode(0644))
	if err != nil {
		return err
	}

	f, err := os.OpenFile(writePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, rc)
	if err != nil {
		return err
	}

	return nil
}
