package backup

import (
	"context"
	"crypto/tls"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/randomcoww/etcd-wrapper/pkg/backup/reader"
	"github.com/randomcoww/etcd-wrapper/pkg/backup/writer"
	"github.com/randomcoww/etcd-wrapper/pkg/util/constants"
)

func FetchBackup(s3Path, downloadPath string) error {
	sess := session.Must(session.NewSession(&aws.Config{
		// Region: aws.String("us-west-2"),
	}))

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultBackupTimeout)
	rm := NewRestoreManagerFromReader(reader.NewS3Reader(s3.New(sess)))
	err := rm.DownloadSnap(ctx, s3Path, downloadPath)

	cancel()
	return err
}

func SendBackup(s3Path string, tlsConfig *tls.Config, clientURLs []string) error {
	sess := session.Must(session.NewSession(&aws.Config{
		// Region: aws.String("us-west-2"),
	}))

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultBackupTimeout)
	bm := NewBackupManagerFromWriter(writer.NewS3Writer(s3.New(sess)), tlsConfig, clientURLs, "")
	_, _, err := bm.SaveSnap(ctx, s3Path, false)

	cancel()
	return err
}
