package util

import (
	"io"
	"os"
	"path/filepath"
)

func WriteFile(rc io.Reader, path string) error {
	err := os.MkdirAll(filepath.Dir(path), os.FileMode(0644))
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
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

func DeleteFile(path string) error {
	return os.Remove(path)
}
