package runner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	s "github.com/randomcoww/etcd-wrapper/internal/snapshot"
	"github.com/randomcoww/etcd-wrapper/internal/util"
	"go.uber.org/zap"
)

type etcdProcess interface {
	StartNew(context.Context, []string) error
	StartExisting(context.Context, []string) error
	Stop() error
}

type Runner struct {
	logger             *zap.Logger
	checkQuorumTimeout time.Duration
	memberTimeout      time.Duration
	etcdRunner         etcdProcess
	etcdConfig         *c.EtcdConfig
	snapshot           *s.Snapshot
}

func NewRunnerFromConfig(logger *zap.Logger, etcdRunner etcdProcess, snapshot *s.Snapshot, raw *c.YamlConfig, etcdConfig *c.EtcdConfig) *Runner {
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
	ok, err := r.checkRevisionAndStart(ctx)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	r.logger.Info("local data not found")

	ok, err = r.checkQuorumAndStart(ctx)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	r.logger.Info("quorum not found")

	ok, err = r.restoreAndStart(ctx)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	return false, nil
}

func (r *Runner) checkRevisionAndStart(ctx context.Context) (bool, error) {
	revision, err := r.getLocalRevision()
	if err != nil {
		return false, fmt.Errorf("init cluster: %w", err)
	}

	if revision > 0 {
		// current revision left on disk prior to any restore
		// if non-zero, assume it is usable and start cluster as is
		r.logger.Info("start etcd as existing")
		if err := r.etcdRunner.StartExisting(ctx, r.etcdConfig.WriteEnv()); err != nil {
			return true, err
		}
		return true, nil
	}

	// pass to next handler
	return false, nil
}

func (r *Runner) checkQuorumAndStart(ctx context.Context) (bool, error) {
	peerClientCtx, cancel := context.WithTimeout(ctx, r.checkQuorumTimeout)
	defer cancel()
	client, err := etcdutil.NewClientFromPeers(peerClientCtx, r.logger, r.etcdConfig.ClusterPeerURLs(), r.etcdConfig.PeerTLSConfig, r.etcdConfig.ClientTLSConfig)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// pass to next handler
			return false, nil
		}
		return false, err
	}
	defer client.Close()

	// checks for quorum
	quorumCtx, cancel := context.WithTimeout(ctx, r.memberTimeout)
	defer cancel()
	remoteRevision, err := client.GetRevision(quorumCtx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// pass to next handler
			return false, nil
		}
		return false, err
	}
	r.logger.Info("remote data", zap.Int64("revision", remoteRevision))

	if err := r.updateMembers(ctx, client); err != nil {
		return false, fmt.Errorf("sync peerURLs: %w", err)
	}
	if err := r.replaceMember(ctx, client); err != nil {
		return false, fmt.Errorf("replace member: %w", err)
	}

	r.logger.Info("start etcd as existing")
	if err := r.etcdRunner.StartExisting(ctx, r.etcdConfig.WriteEnv()); err != nil {
		return true, err
	}
	return true, nil
}

func (r *Runner) restoreAndStart(ctx context.Context) (bool, error) {
	revision, err := r.restoreSnapshot(ctx)
	if err != nil {
		return false, fmt.Errorf("start no quorum: %w", err)
	}

	if revision > 0 {
		r.logger.Info("start etcd as existing")
		if err := r.etcdRunner.StartExisting(ctx, r.etcdConfig.WriteEnv()); err != nil {
			return true, err
		}
		return true, nil
	}

	r.logger.Info("start etcd as new")
	if err := r.etcdRunner.StartNew(ctx, r.etcdConfig.WriteEnv()); err != nil {
		return true, err
	}
	return true, nil
}

func (r *Runner) replaceMember(ctx context.Context, client etcdutil.EtcdClient) error {
	memberListCtx, cancel := context.WithTimeout(ctx, r.memberTimeout)
	defer cancel()
	listResp, err := client.MemberList(memberListCtx)
	if err != nil {
		return fmt.Errorf("list cluster members: %w", err)
	}
	localMember := etcdutil.FindLocalMemberInResp(listResp, r.etcdConfig.Env["ETCD_NAME"], r.etcdConfig.InitialAdvertisePeerURLs)

	// replace my node to join cluster
	// if my node already exists, it needs to be replaced
	if localMember != nil && len(listResp.GetMembers()) >= len(r.etcdConfig.InitialCluster) {
		r.logger.Info("remove local member", zap.Uint64("ID", localMember.GetID()), zap.String("name", localMember.GetName()))

		memberRemoveCtx, cancel := context.WithTimeout(ctx, r.memberTimeout)
		defer cancel()
		listResp, err = client.MemberRemove(memberRemoveCtx, localMember.GetID())
		if err != nil {
			return fmt.Errorf("remove cluster member %d: %w", localMember.GetID(), err)
		}
		localMember = etcdutil.FindLocalMemberInResp(listResp, r.etcdConfig.Env["ETCD_NAME"], r.etcdConfig.InitialAdvertisePeerURLs)

		// local data, if any is not usable now
		if err := r.clearLocalData(); err != nil {
			return fmt.Errorf("clear local data: %w", err)
		}
	}

	if localMember == nil && len(listResp.GetMembers()) < len(r.etcdConfig.InitialCluster) {
		memberAddCtx, cancel := context.WithTimeout(ctx, r.memberTimeout)
		defer cancel()
		listResp, err = client.MemberAdd(memberAddCtx, r.etcdConfig.InitialAdvertisePeerURLs)
		if err != nil {
			return fmt.Errorf("add member: %w", err)
		}
		localMember = etcdutil.FindLocalMemberInResp(listResp, r.etcdConfig.Env["ETCD_NAME"], r.etcdConfig.InitialAdvertisePeerURLs)
		r.logger.Info("added local member", zap.Uint64("ID", localMember.GetID()), zap.String("name", localMember.GetName()))
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
	if err := util.RemoveDir(r.etcdConfig.Env["ETCD_DATA_DIR"]); err != nil {
		return fmt.Errorf("remove data dir: %w", err)
	}
	if err := util.RemoveDir(r.etcdConfig.Env["ETCD_WAL_DIR"]); err != nil {
		return fmt.Errorf("remove data dir: %w", err)
	}
	r.logger.Info("cleaned out existing data")
	return nil
}

func (r *Runner) getLocalRevision() (int64, error) {
	revision, err := etcdutil.GetDataRevision(r.etcdConfig.Env["ETCD_DATA_DIR"], r.logger)
	if err != nil {
		if err := r.clearLocalData(); err != nil {
			return 0, fmt.Errorf("get local revision: %w", err)
		}
	}
	r.logger.Info("local data", zap.Int64("revision", revision))
	return revision, nil
}

func (r *Runner) restoreSnapshot(ctx context.Context) (int64, error) {
	if err := r.clearLocalData(); err != nil {
		return 0, err
	}
	ok, err := r.snapshot.VerifyBucket(ctx)
	if err != nil {
		return 0, fmt.Errorf("verify backup bucket: %w", err)
	}
	if !ok {
		return 0, fmt.Errorf("verify backup bucket")
	}
	ok, err = r.snapshot.Restore(ctx, r.etcdConfig.RestoreConfig())
	if err != nil {
		return 0, fmt.Errorf("restore snapshot: %w", err)
	}
	if ok {
		return r.getLocalRevision()
	}
	return 0, nil
}
