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

func HasMatchingElement[T comparable](s1, s2 []T) {
	elementsInS1 := make(map[T]struct{})
	for _, v := range s1 {
		elementsInS1[v] = struct{}{}
	}
	for _, v := range s2 {
		if _, ok := elementsInS1[v]; ok {
			return true
		}
	}
	return false
}
