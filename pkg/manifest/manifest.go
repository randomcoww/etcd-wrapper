package manifest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/podspec"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	"io"
	"k8s.io/api/core/v1"
	"log"
)

type Manifest interface {
	WriteFile(args *arg.Args) error
	DeleteFile(args *arg.Args) error
}

type EtcdPod struct{}

func (p *EtcdPod) WriteFile(args *arg.Args) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var pod *v1.Pod
	var ok bool
	var err error
	var manifest []byte

	switch args.InitialClusterState {
	case "new":
		if ok, err = args.S3Client.Download(ctx, args.S3BackupBucket, args.S3BackupKey, func(ctx context.Context, r io.Reader) error {
			return util.WriteFile(r, args.EtcdSnapshotFile)
		}); err != nil {
			return fmt.Errorf("Error getting snapshot: %v", err)
		}
		if !ok {
			log.Printf("Snapshot not found. Starting new cluster")
			if pod, err = podspec.Create(args, false); err != nil {
				return fmt.Errorf("Error creating pod manifest: %v", err)
			}
		} else {
			log.Printf("Successfully got snapshot. Restoring existing cluster")
			if pod, err = podspec.Create(args, true); err != nil {
				return fmt.Errorf("Error creating pod manifest: %v", err)
			}
		}
	case "existing":
		if pod, err = podspec.Create(args, false); err != nil {
			return fmt.Errorf("Error creating pod manifest: %v", err)
		}
	default:
		return fmt.Errorf("InitialClusterState not defined")
	}
	if manifest, err = json.MarshalIndent(pod, "", "  "); err != nil {
		return err
	}
	return util.WriteFile(io.NopCloser(bytes.NewReader(manifest)), args.EtcdPodManifestWritePath)
}

func (p *EtcdPod) DeleteFile(args *arg.Args) error {
	return util.DeleteFile(args.EtcdPodManifestWritePath)
}
