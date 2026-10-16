package util

import (
	"fmt"
	"os"
)

func HasMatchingElement[T comparable](s1, s2 []T) bool {
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

func RemoveDir(path string) error {
	_, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return fmt.Errorf("remove path %s: %w", path, err)
	}
	return os.RemoveAll(path)
}
