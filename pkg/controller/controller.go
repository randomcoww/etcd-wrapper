package controller

import (
	"context"
	c "github.com/randomcoww/etcd-wrapper/pkg/config"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdclient"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdprocess"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"time"
)

type Controller struct {
	P etcdprocess.EtcdProcess
}

const (
	backoffWaitBetween time.Duration = 2 * time.Second
)

func NewController(ctx context.Context, config *c.Config) (*Controller, error) {
	return &Controller{
		P: etcdprocess.NewProcess(ctx, config),
	}, nil
}

func (c *Controller) Run(ctx context.Context, config *c.Config) error {
	config.Env["ETCD_INITIAL_CLUSTER_STATE"] = "existing"
	client, err := etcdclient.NewClientFromPeers(ctx, config)
	if err == nil {
		if err = client.GetHealth(ctx); err == nil {
			list, err := client.MemberList(ctx)
			if err == nil {
				var localMember *etcdserverpb.Member
				for _, member := range list.GetMembers() {
					if util.HasMatchingElement(member.GetPeerURLs(), config.InitialAdvertisePeerURLs) {
						localMember = member
						break
					}
				}

				if localMember != nil {
					status, err := client.Status(ctx, config.ListenClientURLs[0])
					if err == nil {
						if status.GetHeader().GetClusterId() == localMember.GetID() {
							return nil // nothing to do
						}
					}
					if len(list.GetMembers()) >= len(config.ClusterPeerURLs) {
						list, err = client.MemberRemove(ctx, localMember.GetID())
						if err != nil {
							return err
						}
					}
					if err = c.P.Stop(); err != nil {
						return err
					}
					return etcdprocess.RemoveDataDir(config)
				}

				if len(list.GetMembers()) < len(config.ClusterPeerURLs) {
					list, err = client.MemberAdd(ctx, config.InitialAdvertisePeerURLs)
					if err != nil {
						return err
					}
				}
				return c.P.Reconfigure(config)
			}
		}
	}

	/*
		if err = etcdprocess.RestoreV3Snapshot(ctx, config, snapshotFile); err != nil {
			return err
		}
	*/
	ok, err := etcdprocess.DataExists(config)
	if err != nil {
		return err
	}
	if !ok {
		config.Env["ETCD_INITIAL_CLUSTER_STATE"] = "new"
	}
	return c.P.Reconfigure(config)
}
