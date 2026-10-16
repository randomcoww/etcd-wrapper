package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	s "github.com/randomcoww/etcd-wrapper/internal/snapshot"
	"go.uber.org/zap"
)

const (
	timeFormat string = "20060102-150405"
)

type Backup struct {
	logger             *zap.Logger
	backupInterval     time.Duration
	checkQuorumTimeout time.Duration
	memberTimeout      time.Duration
	etcdConfig         *c.EtcdConfig
	snapshot           *s.Snapshot
	Success            bool
}

type node struct {
	member   etcdutil.Member
	status   etcdutil.Status
	isLeader bool
}

func (b *Backup) StartLoop(ctx context.Context) error {
	ticker := time.NewTicker(b.backupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := b.run(ctx); err != nil {
				b.Success = false
				b.logger.Error("backup loop", zap.Error(err))
				continue
			}
			b.Success = true
		case <-ctx.Done():
			return nil
		}
	}
}

func (b *Backup) run(ctx context.Context) error {
	client, err := b.clientWithQuorum(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	localNode, err := b.getLocalNode(ctx, client)
	if err != nil {
		return err
	}
	b.logger.Info("found local node", zap.Uint64("ID", localNode.member.GetID()))

	// --- defrag ---

	defragmentCtx, cancel := context.WithTimeout(ctx, b.memberTimeout)
	defer cancel()
	err = client.Defragment(defragmentCtx, localNode.member.GetClientURLs()[0])
	if err != nil {
		return err
	}
	b.logger.Info("ran defragment")

	// --- run backup as leader ---

	if !localNode.isLeader {
		b.logger.Info("not leader, skipping backup")
		return nil
	}

	snapshotCtx, cancel := context.WithTimeout(ctx, b.memberTimeout)
	defer cancel()
	reader, err := client.Snapshot(snapshotCtx)
	if err != nil {
		return err
	}

	err = b.snapshot.CreateBackup(ctx, reader)
	if err != nil {
		return err
	}
	return nil
}

func (b *Backup) clientWithQuorum(ctx context.Context) (etcdutil.EtcdClient, error) {
	peerClientCtx, cancel := context.WithTimeout(ctx, b.checkQuorumTimeout)
	defer cancel()
	client, err := etcdutil.NewClientFromPeers(peerClientCtx, b.logger, b.etcdConfig.ClusterPeerURLs(), b.etcdConfig.PeerTLSConfig, b.etcdConfig.ClientTLSConfig)
	if err != nil {
		return nil, err
	}
	if err := client.CheckQuorum(peerClientCtx); err != nil {
		client.Close()
		return nil, err
	}
	return client, nil
}

func (b *Backup) getLocalNode(ctx context.Context, client etcdutil.EtcdClient) (*node, error) {
	memberListCtx, cancel := context.WithTimeout(ctx, b.memberTimeout)
	defer cancel()
	listResp, err := client.MemberList(memberListCtx)
	if err != nil {
		return nil, fmt.Errorf("list cluster members: %w", err)
	}

	if len(listResp.GetMembers()) < len(b.etcdConfig.InitialCluster) {
		return nil, fmt.Errorf("not enough members responding")
	}
	b.logger.Info("found members", zap.Int("count", len(listResp.GetMembers())))

	var clientURLs []string
	for _, member := range listResp.GetMembers() {
		clientURLs = append(clientURLs, member.GetClientURLs()...)
	}

	statusMap, err := b.statusMap(ctx, client, clientURLs)
	if err != nil {
		return nil, fmt.Errorf("one or more status responses failed: %w", err)
	}

	localMember := etcdutil.FindLocalMemberInResp(listResp, b.etcdConfig.Env["ETCD_NAME"], b.etcdConfig.InitialAdvertisePeerURLs)
	if localMember == nil {
		return nil, fmt.Errorf("local member not found")
	}
	localStatus, ok := statusMap[localMember.GetID()]
	if !ok {
		return nil, fmt.Errorf("local status response not found")
	}
	b.logger.Info("all members respond to status")

	return &node{
		member:   localMember,
		status:   localStatus,
		isLeader: localStatus.GetLeader() == localMember.GetID(),
	}, nil
}

func (b *Backup) statusMap(ctx context.Context, client etcdutil.EtcdClient, endpoints []string) (map[uint64]etcdutil.Status, error) {
	statusCtx, cancel := context.WithTimeout(ctx, b.memberTimeout)
	defer cancel()

	statusCh := make(chan etcdutil.Status)
	errCh := make(chan error)

	for _, url := range endpoints {
		go func() {
			status, err := client.Status(statusCtx, url)
			if err != nil {
				errCh <- err
			} else {
				statusCh <- status
			}
		}()
	}

	res := make(map[uint64]etcdutil.Status)
	errs := []error{}

	for i := 0; i < len(endpoints); i++ {
		select {
		case status := <-statusCh:
			memberID := status.GetHeader().GetMemberId()
			if memberID != 0 {
				res[memberID] = status
			}
		case err := <-errCh:
			errs = append(errs, err)
		}
	}
	return res, errors.Join(errs...)
}
