package runner

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	s "github.com/randomcoww/etcd-wrapper/internal/snapshot"
	"github.com/randomcoww/etcd-wrapper/internal/util"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.uber.org/zap"
)

const (
	defaultRevision int64 = 1
)

type etcdProcess interface {
	StartNew([]string) error
	StartExisting([]string) error
	Stop() error
	Wait() error
}

type Runner struct {
	logger             *zap.Logger
	checkQuorumTimeout time.Duration
	memberTimeout      time.Duration
	etcdRunner         etcdProcess
	etcdConfig         *c.EtcdConfig
	snapshot           *s.Snapshot
}

func NewFromConfig(logger *zap.Logger, etcdRunner etcdProcess, snapshot *s.Snapshot, raw *c.YamlConfig, etcdConfig *c.EtcdConfig) *Runner {
	return &Runner{
		logger:             logger,
		checkQuorumTimeout: raw.CheckQuorumDelay,
		memberTimeout:      raw.MemberTimeout,
		etcdRunner:         etcdRunner,
		etcdConfig:         etcdConfig,
		snapshot:           snapshot,
	}
}

func (r *Runner) StartInit(ctx context.Context) (bool, error) {
	ok, err := r.checkQuorumAndStart(ctx)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	r.logger.Info("quorum not found")

	ok, err = r.startNoQuorum(ctx)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	return false, nil
}

func (r *Runner) StartLoop(ctx context.Context, start chan struct{}) error {
	for {
		ok, err := r.checkQuorumAndStart(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		r.logger.Info("quorum not found")

		select {
		case <-start:
			return r.etcdRunner.StartNew(r.etcdConfig.WriteEnv())
		case <-ctx.Done():
			return nil
		}
	}
}

func (r *Runner) checkQuorumAndStart(ctx context.Context) (bool, error) {
	peerClientCtx, cancel := context.WithTimeout(ctx, r.checkQuorumTimeout)
	defer cancel()
	client, err := etcdutil.NewClientFromPeers(peerClientCtx, r.logger, r.etcdConfig.ClusterPeerURLs(), r.etcdConfig.PeerTLSConfig, r.etcdConfig.ClientTLSConfig)
	if err == nil {
		defer client.Close()
		err = client.CheckQuorum(peerClientCtx)
		if err == nil {
			if err := r.addMemberToQuorum(ctx, client); err != nil {
				return true, err
			}
			return true, nil
		}
	}
	return false, nil
}

func (r *Runner) startNoQuorum(ctx context.Context) (bool, error) {
	revision, err := r.getLocalRevision()
	if err != nil {
		return false, fmt.Errorf("init cluster: %w", err)
	}
	if revision == defaultRevision {
		revision, err = r.restoreSnapshot(ctx)
		if err != nil {
			return false, fmt.Errorf("init cluster: %w", err)
		}
		if revision == defaultRevision {
			return false, nil
		}
	}
	r.logger.Info("start etcd existing")
	if err := r.etcdRunner.StartExisting(r.etcdConfig.WriteEnv()); err != nil {
		return true, err
	}
	return true, nil
}

func (r *Runner) addMemberToQuorum(ctx context.Context, client etcdutil.EtcdClient) error {
	r.logger.Info("existing quorum found")

	if err := r.updateMembers(ctx, client); err != nil {
		return fmt.Errorf("sync peerURLs: %w", err)
	}
	if err := r.replaceMember(ctx, client); err != nil {
		return fmt.Errorf("replace member: %w", err)
	}
	r.logger.Info("replaced local member")

	if err := r.clearLocalData(); err != nil {
		return fmt.Errorf("clear local data: %w", err)
	}
	r.logger.Info("cleared local data")

	r.logger.Info("start etcd existing")
	return r.etcdRunner.StartExisting(r.etcdConfig.WriteEnv())
}

func (r *Runner) replaceMember(ctx context.Context, client etcdutil.EtcdClient) error {
	memberListCtx, cancel := context.WithTimeout(ctx, r.memberTimeout)
	defer cancel()
	listResp, err := client.MemberList(memberListCtx)
	if err != nil {
		return fmt.Errorf("list cluster members: %w", err)
	}
	localMember := findMemberInCluster(listResp, r.etcdConfig.Env["ETCD_NAME"], r.etcdConfig.InitialAdvertisePeerURLs)

	// replace my node to join cluster
	// if my node already exists, it needs to be replaced
	if localMember != nil && len(listResp.GetMembers()) >= len(r.etcdConfig.InitialCluster) {
		r.logger.Info("remove local member", zap.Uint64("ID", localMember.GetID()))

		memberRemoveCtx, cancel := context.WithTimeout(ctx, r.memberTimeout)
		defer cancel()
		listResp, err = client.MemberRemove(memberRemoveCtx, localMember.GetID())
		if err != nil {
			return fmt.Errorf("remove cluster member %d: %w", localMember.GetID(), err)
		}
		localMember = findMemberInCluster(listResp, r.etcdConfig.Env["ETCD_NAME"], r.etcdConfig.InitialAdvertisePeerURLs)
		r.logger.Info("removed local member")
	}

	if localMember == nil && len(listResp.GetMembers()) < len(r.etcdConfig.InitialCluster) {
		memberAddCtx, cancel := context.WithTimeout(ctx, r.memberTimeout)
		defer cancel()
		listResp, err = client.MemberAdd(memberAddCtx, r.etcdConfig.InitialAdvertisePeerURLs)
		if err != nil {
			return fmt.Errorf("add member: %w", err)
		}
		localMember = findMemberInCluster(listResp, r.etcdConfig.Env["ETCD_NAME"], r.etcdConfig.InitialAdvertisePeerURLs)
		r.logger.Info("added local member", zap.Uint64("ID", localMember.GetID()))
	}
	return nil
}

func (r *Runner) updateMembers(ctx context.Context, client etcdutil.EtcdClient) error {
	memberListCtx, cancel := context.WithTimeout(ctx, r.memberTimeout)
	defer cancel()
	listResp, err := client.MemberList(memberListCtx)
	if err != nil {
		return fmt.Errorf("list cluster members: %w", err)
	}

	for _, member := range listResp.GetMembers() {
		switch {
		case member.GetID() == 0:
		case member.GetName() == r.etcdConfig.Env["ETCD_NAME"], util.HasMatchingElement(member.GetPeerURLs(), r.etcdConfig.InitialAdvertisePeerURLs):
			if !slices.Equal(member.GetPeerURLs(), r.etcdConfig.InitialAdvertisePeerURLs) {

				memberUpdateCtx, cancel := context.WithTimeout(ctx, r.memberTimeout)
				defer cancel()
				listResp, err = client.MemberUpdate(memberUpdateCtx, member.GetID(), r.etcdConfig.InitialAdvertisePeerURLs)
				if err != nil {
					return err
				}
				r.logger.Info("update local member", zap.Uint64("ID", member.GetID()), zap.String("peerURLs", fmt.Sprintf("%+v", r.etcdConfig.InitialAdvertisePeerURLs)))
			}
			r.etcdConfig.InitialCluster[r.etcdConfig.Env["ETCD_NAME"]] = r.etcdConfig.InitialAdvertisePeerURLs

		case member.GetName() == "":
		default:
			memberPeerURLs, ok := r.etcdConfig.InitialCluster[member.GetName()]
			if !ok {
				break
			}

			peerURLsMap := make(map[string]struct{})
			for _, u := range member.GetPeerURLs() {
				peerURLsMap[u] = struct{}{}
			}

			var newMemberPeerURLs []string
			for _, memberPeerURL := range memberPeerURLs {
				if _, ok := peerURLsMap[memberPeerURL]; !ok {
					newMemberPeerURLs = append(newMemberPeerURLs, memberPeerURL)
				}
			}
			if len(newMemberPeerURLs) > 0 {
				newMemberPeerURLs = append(newMemberPeerURLs, member.GetPeerURLs()...)

				memberUpdateCtx, cancel := context.WithTimeout(ctx, r.memberTimeout)
				defer cancel()
				_, err := client.MemberUpdate(memberUpdateCtx, member.GetID(), newMemberPeerURLs)
				if err != nil {
					return err
				}
				r.etcdConfig.InitialCluster[member.GetName()] = newMemberPeerURLs
				r.logger.Info("update member", zap.Uint64("ID", member.GetID()), zap.String("peerURLs", fmt.Sprintf("%+v", newMemberPeerURLs)))
			}
		}
	}
	return nil
}

func (r *Runner) clearLocalData() error {
	if err := removeDir(r.etcdConfig.Env["ETCD_DATA_DIR"]); err != nil {
		return fmt.Errorf("remove data dir: %w", err)
	}
	if err := removeDir(r.etcdConfig.Env["ETCD_WAL_DIR"]); err != nil {
		return fmt.Errorf("remove data dir: %w", err)
	}
	r.logger.Info("cleaned out existing data")
	return nil
}

func (r *Runner) getLocalRevision() (int64, error) {
	revision, err := etcdutil.GetDataRevision(r.etcdConfig.Env["ETCD_DATA_DIR"], r.logger)
	if err != nil {
		r.logger.Error("get local data revision", zap.Error(err))
		if err := r.clearLocalData(); err != nil {
			return defaultRevision, fmt.Errorf("clear local data: %w", err)
		}
		r.logger.Info("cleared local data")
	}
	if revision == 0 {
		revision = defaultRevision
	}
	r.logger.Info("local data", zap.Int64("revision", revision))
	return revision, nil
}

func (r *Runner) restoreSnapshot(ctx context.Context) (int64, error) {
	if err := r.clearLocalData(); err != nil {
		return defaultRevision, err
	}
	ok, err := r.snapshot.VerifyBucket(ctx)
	if err != nil {
		return defaultRevision, fmt.Errorf("verify backup bucket: %w", err)
	}
	if !ok {
		return defaultRevision, fmt.Errorf("verify backup bucket")
	}
	ok, err = r.snapshot.Restore(ctx, r.etcdConfig.RestoreConfig())
	if err != nil {
		return defaultRevision, fmt.Errorf("restore snapshot: %w", err)
	}
	if ok {
		return r.getLocalRevision()
	}
	return defaultRevision, nil
}

func findMemberInCluster(listResp etcdutil.Members, name string, peerURLs []string) *etcdserverpb.Member {
	for _, member := range listResp.GetMembers() {
		if member.GetName() == name {
			return member
		}
		if util.HasMatchingElement(member.GetPeerURLs(), peerURLs) {
			return member
		}
	}
	return nil
}

func removeDir(path string) error {
	_, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return fmt.Errorf("remove path %s: %w", path, err)
	}
	return os.RemoveAll(path)
}
