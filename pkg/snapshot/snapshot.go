package snapshot

import (
	// "context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/randomcoww/etcd-wrapper/pkg/util/s3util"
	"io"
	"os"
	"path/filepath"
)

func Restore(s3resource string, restoreFile string) error {
	sess := session.Must(session.NewSession(&aws.Config{}))
	reader := s3util.NewReader(s3.New(sess))
	readCloser, err := reader.Open(s3resource)
	if err != nil {
		return err
	}
	defer readCloser.Close()

	err = os.MkdirAll(filepath.Dir(restoreFile), os.FileMode(0644))
	if err != nil {
		return err
	}

	f, err := os.OpenFile(restoreFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, readCloser)
	if err != nil {
		return err
	}

	info, err := os.Stat(restoreFile)
	if err != nil {
		return err
	}

	if info.Size() == 0 {
		return fmt.Errorf("snapshot file size is zero")
	}
	return nil
}

func Backup(s3resource string) error {
	// ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	// readCloser, err := etcdcli.Snapshot(ctx)
	// if err != nil {
	// 	return err
	// }
	// defer readCloser.Close()

	// sess := session.Must(session.NewSession(&aws.Config{}))
	// writer := s3util.NewWriter(s3.New(sess))
	// _, err := writer.Write(ctx, s3Resource, readCloser)
	// cancel()
	// return err
	return nil
}
