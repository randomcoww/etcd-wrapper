package s3test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

type FakeS3Client struct {
	ObjectsList       []string
	ObjectsErrors     map[string]error
	SnapshotFiles     map[string]string
	TestEncryptionKey string
}

func (c *FakeS3Client) VerifyBucket(ctx context.Context) (bool, error) {
	return true, nil
}

func (c *FakeS3Client) Download(ctx context.Context, key string) (io.ReadCloser, bool, error) {
	file, err := os.Open(c.SnapshotFiles[key])
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	var buf bytes.Buffer

	// --- simluate encrypted file ---
	recipient, err := age.NewScryptRecipient(c.TestEncryptionKey)
	if err != nil {
		return nil, false, fmt.Errorf("create scrypt recipient: %w", err)
	}
	ageWriter, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, false, fmt.Errorf("encryption writer: %w", err)
	}
	_, err = io.Copy(ageWriter, file)
	if err := ageWriter.Close(); err != nil {
		return nil, false, fmt.Errorf("close age writer: %w", err)
	}
	// ---

	return io.NopCloser(&buf), true, nil
}

func (c *FakeS3Client) List(ctx context.Context, prefix string) ([]string, map[string]error) {
	return c.ObjectsList, c.ObjectsErrors
}
