package controller

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/s3client"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"io"
	"os"
	"time"
)

type Controller struct {
	P              etcdprocess.EtcdProcess
	S3Client       s3client.Client
	peerTimeout    time.Duration
	clientTimeout  time.Duration
	restoreTimeout time.Duration
}

func (c *Controller) RunEtcd(ctx context.Context, config *c.Config) error {
	peerCtx, _ := context.WithTimeout(context.Background(), time.Duration(c.peerTimeout))
	client, err := etcdclient.NewClientFromPeers(peerCtx, config)
	if err == nil {
		clientCtx, _ := context.WithTimeout(context.Background(), time.Duration(c.clientTimeout))
		list, err := client.MemberList(clientCtx)
		if err == nil {
			var localMember *etcdserverpb.Member
			for _, member := range list.GetMembers() {
				if util.HasMatchingElement(member.GetPeerURLs(), config.InitialAdvertisePeerURLs) {
					localMember = member
					break
				}
			}
			if localMember != nil {
				if len(list.GetMembers()) < len(config.ClusterPeerURLs) {
					list, err = client.MemberAdd(clientCtx, config.InitialAdvertisePeerURLs)
					if err != nil {
						return err
					}
				}
			}
		}
	} else {
		ok, err := etcdprocess.DataExists(config)
		if err != nil {
			return err
		}
		if !ok {
			restoreCtx, _ := context.WithTimeout(context.Background(), time.Duration(c.restoreTimeout))
			ok, err = c.restoreSnapshot(restoreCtx, config)
			if err != nil {
				return err
			}
		}
	}
	return c.P.Start()
}

func (c *Controller) restoreSnapshot(ctx context.Context, config *c.Config) (bool, error) {
	snapshotFile, err := os.CreateTemp("", "snapshot*.db")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(snapshotFile.Name())
	defer snapshotFile.Close()

	ok, err := c.S3Client.Download(ctx, config.S3BackupBucket, config.S3BackupKey, func(ctx context.Context, reader io.Reader) (bool, error) {
		b, err := io.Copy(snapshotFile, reader)
		if err != nil {
			return false, err
		}
		return b > 0, nil
	})
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if err := etcdprocess.RestoreV3Snapshot(ctx, config, snapshotFile.Name()); err != nil {
		return false, err
	}
	return true, nil
}
