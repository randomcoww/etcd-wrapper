package snapshot

// import (
// 	"context"
// 	"fmt"
// 	"github.com/randomcoww/etcd-wrapper/pkg/s3util"
// 	"io"
// 	"os"
// 	"path/filepath"
// 	"context"
// 	"crypto/tls"
// 	"github.com/aws/aws-sdk-go/aws"
// 	"github.com/aws/aws-sdk-go/aws/session"
// 	"github.com/aws/aws-sdk-go/service/s3"
// )

// func Create() error {

// }

// func Download(s3Path, writePath string) error {
// 	sess := session.Must(session.NewSession(&aws.Config{}))
// 	reader := s3util.NewReader(s3.New(sess))

// 	rc, err := reader.Open()
// 	if err != nil {
// 		return err
// 	}
// 	util.WriteFile(rc, writePath)

// 	stat, err = os.Stat(writePath)
// 	if err != nil {
// 		return err
// 	}

// 	if stat.Size() == 0 {
// 		return fmt.Errorf("restored snapshot file %s is empty", writePath)
// 	}
// 	return nil
// }
