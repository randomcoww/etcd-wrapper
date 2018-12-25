package backup

import (
	"context"
	"crypto/tls"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	importbackup "github.com/coreos/etcd-operator/pkg/backup"
	"github.com/coreos/etcd-operator/pkg/backup/reader"
	"github.com/coreos/etcd-operator/pkg/backup/writer"
	"github.com/coreos/etcd-operator/pkg/util/constants"
)

func FetchBackup(s3Path, downloadPath string) error {
	sess := session.Must(session.NewSession(&aws.Config{
		// Region: aws.String("us-west-2"),
	}))

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	s3Reader := reader.NewS3Reader(s3.New(sess))
	rm := NewRestoreManagerFromReader(s3Reader)
	err := rm.DownloadSnap(ctx, s3Path, downloadPath)

	cancel()
	return err
}

func SendBackup(s3Path string, tlsConfig *tls.Config, clientURLs []string) error {
	sess := session.Must(session.NewSession(&aws.Config{
		// Region: aws.String("us-west-2"),
	}))

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	s3Writer := writer.NewS3Writer(s3.New(sess))
	bm := importbackup.NewBackupManagerFromWriter(nil, s3Writer, tlsConfig, clientURLs, "")
	_, _, err := bm.SaveSnap(ctx, s3Path)

	cancel()
	return err
}
