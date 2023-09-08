package reader

import "io"

// Reader defines required reader operations
type Reader interface {
	// Open opens up a backup file for reading.
	Open(path string) (rc io.ReadCloser, err error)
}
