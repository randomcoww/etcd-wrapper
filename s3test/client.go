package s3test

import (
	"bytes"
	"context"
	"filippo.io/age"
	"fmt"
	"io"
	"os"
	"slices"
)

type FakeS3Client struct {
	TempDir           string
	ObjectsList       []string
	ObjectsErrors     map[string]error
	SnapshotFiles     map[string]string
	RemoveCalled      []string
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

	// --- download decrypts file, ensure this works with encrypted file ---

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

	return io.NopCloser(&buf), true, nil
}

func (c *FakeS3Client) List(ctx context.Context, prefix string) ([]string, map[string]error) {
	slices.Sort(c.ObjectsList)
	return c.ObjectsList, c.ObjectsErrors
}

func (c *FakeS3Client) Remove(ctx context.Context, keys []string) error {
	c.RemoveCalled = append(c.RemoveCalled, keys...)
	return nil
}

func (c *FakeS3Client) Upload(ctx context.Context, key string, reader io.Reader) error {
	putFile, err := os.CreateTemp(c.TempDir, "snapshot-put-*.db")
	if err != nil {
		return fmt.Errorf("open file for put: %w", err)
	}
	defer putFile.Close()

	// --- file is encrypted during upload, test that it decrypts ---

	identity, err := age.NewScryptIdentity(c.TestEncryptionKey)
	if err != nil {
		return fmt.Errorf("create scrypt identity: %w", err)
	}
	decryptedReader, err := age.Decrypt(reader, identity)
	if err != nil {
		return fmt.Errorf("decryption writer: %w", err)
	}
	b, err := io.Copy(putFile, decryptedReader)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	if b == 0 {
		return fmt.Errorf("upload size is 0")
	}

	c.SnapshotFiles[key] = putFile.Name()
	c.ObjectsList = append(c.ObjectsList, key)
	return nil
}
