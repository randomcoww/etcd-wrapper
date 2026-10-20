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

type Backup struct {
	logger             *zap.Logger
	backupInterval     time.Duration
	checkQuorumTimeout time.Duration
	memberTimeout      time.Duration
	snapshotTimeout    time.Duration
	defragmentTimeout  time.Duration
	etcdConfig         *c.EtcdConfig
	snapshot           *s.Snapshot
	defragThreshold    float64
	Error              error
}

type node struct {
	member   etcdutil.Member
	status   etcdutil.Status
	isLeader bool
	isLocal  bool
}

func NewBackupFromConfig(logger *zap.Logger, snapshot *s.Snapshot, raw *c.YamlConfig, etcdConfig *c.EtcdConfig) *Backup {
	return &Backup{
		logger:             logger,
		backupInterval:     raw.BackupInterval,
		checkQuorumTimeout: raw.CheckQuorumTimeout,
		memberTimeout:      raw.MemberTimeout,
		snapshotTimeout:    raw.SnapshotTimeout,
		defragmentTimeout:  raw.DefragmentTimeout,
		etcdConfig:         etcdConfig,
		snapshot:           snapshot,
		defragThreshold:    raw.DefragThreshold,
	}
}

func (b *Backup) StartLoop(ctx context.Context) error {
	ticker := time.NewTicker(b.backupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.Error = b.run(ctx)
			if b.Error != nil {
				b.logger.Error("backup loop", zap.Error(b.Error))
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (b *Backup) run(ctx context.Context) error {
	client, err := b.clientWithQuorum(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	nodes, err := b.nodeList(ctx, client)
	if err != nil {
		return err
	}

	var localLeader *node
	for _, node := range nodes {
		if node.isLocal && node.isLeader {
			localLeader = node
		}
	}
	if localLeader == nil {
		return nil
	}

	b.logger.Info("node is leader - running maintenance")

	// --- defrag ---

	if err := b.rollingDefragment(ctx, client, nodes); err != nil {
		return err
	}
	b.logger.Info("ran rolling defragment")

	// --- snapshot backup ---

	snapshotCtx, cancel := context.WithTimeout(ctx, b.snapshotTimeout)
	defer cancel()
	reader, err := client.Snapshot(snapshotCtx)
	if err != nil {
		return err
	}
	defer reader.Close()
	b.logger.Info("created snapshot backup")

	if err := b.snapshot.CreateBackup(ctx, reader); err != nil {
		return err
	}
	b.logger.Info("uploaded snapshot backup")

	if err := b.snapshot.RemoveOldBackups(ctx); err != nil {
		return err
	}
	b.logger.Info("removed old snapshots")

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

func (b *Backup) nodeList(ctx context.Context, client etcdutil.EtcdClient) ([]*node, error) {
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

	res := []*node{}
	for _, member := range listResp.GetMembers() {
		if member.GetName() == "" {
			return nil, fmt.Errorf("member name not set")
		}
		status, ok := statusMap[member.GetID()]
		if !ok {
			return nil, fmt.Errorf("local status response not found")
		}
		res = append(res, &node{
			status:   status,
			member:   member,
			isLeader: status.GetLeader() == member.GetID(),
			isLocal:  b.etcdConfig.Env["ETCD_NAME"] == member.GetName(),
		})
	}
	return res, nil
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

func (b *Backup) rollingDefragment(ctx context.Context, client etcdutil.EtcdClient, nodeList []*node) error {
	defragmentNodes := []*node{}

	for _, n := range nodeList {
		if !b.needsDefragment(n.status) {
			continue
		}
		if n.isLeader {
			defragmentNodes = append(defragmentNodes, n)
			continue
		}
		defragmentNodes = append([]*node{n}, defragmentNodes...)
	}

	for _, n := range defragmentNodes {
		if err := b.runDefragment(ctx, n.member, client); err != nil {
			return err
		}
	}
	return nil
}

func (b *Backup) runDefragment(ctx context.Context, member etcdutil.Member, client etcdutil.EtcdClient) error {
	defragmentCtx, cancel := context.WithTimeout(ctx, b.defragmentTimeout)
	defer cancel()
	b.logger.Info("start defragment", zap.String("node", member.GetName()))
	if err := client.Defragment(defragmentCtx, member.GetClientURLs()[0]); err != nil {
		return err
	}

	// wait for status to respond
	time.Sleep(1 * time.Second)
	if _, err := client.Status(defragmentCtx, member.GetClientURLs()[0]); err != nil {
		return err
	}
	return nil
}

func (b *Backup) needsDefragment(status etcdutil.Status) bool {
	if status.GetDbSize() == 0 {
		return false
	}
	fragmentation := (float64(status.GetDbSize()-status.GetDbSizeInUse()) / float64(status.GetDbSize()))
	if fragmentation >= b.defragThreshold {
		b.logger.Info("defragment needed", zap.Float64("ratio", fragmentation))
		return true
	}
	b.logger.Info("skip defragment", zap.Float64("ratio", fragmentation))
	return false
}
