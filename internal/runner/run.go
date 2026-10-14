package runner

import (
	"context"
	"fmt"
	"os"
	"slices"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	"github.com/randomcoww/etcd-wrapper/internal/s3util"
	"github.com/randomcoww/etcd-wrapper/internal/snapshot"
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
	logger                 *zap.Logger
	checkQuorumTimeout     time.Duration
	addQuorumMemberTimeout time.Duration
	restoreSnapshotTimeout time.Duration
	etcdRunner             etcdProcess
	envConfig              *c.EnvConfig
	s3Client               s3util.Client
	objectPrefix           string
	SnapshotEncryptionKey  string
	restoreRevisionBump    uint64
}

func (r *Runner) init(ctx context.Context) (bool, error) {
	peerClientCtx, cancel := context.WithTimeout(ctx, r.checkQuorumTimeout)
	defer cancel()
	client, err := etcdutil.NewClientFromPeers(peerClientCtx, r.logger, r.envConfig.ClusterPeerURLs(), r.envConfig.PeerTLSConfig, r.envConfig.ClientTLSConfig)
	if err == nil {
		defer client.Close()
		err = client.CheckQuorum(peerClientCtx)
		if err == nil {
			addQuorumMemberCtx, cancel := context.WithTimeout(ctx, r.addQuorumMemberTimeout)
			defer cancel()
			if err := r.addMemberToQuorum(addQuorumMemberCtx, client); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	r.logger.Info("quorum not found")
	revision, err := r.getLocalRevision()
	if err != nil {
		return false, fmt.Errorf("init cluster: %w", err)
	}
	if revision == defaultRevision {
		restoreSnapshotCtx, cancel := context.WithTimeout(ctx, r.restoreSnapshotTimeout)
		defer cancel()
		revision, err = r.restoreSnapshot(restoreSnapshotCtx)
		if err != nil {
			return false, fmt.Errorf("init cluster: %w", err)
		}
		if revision == defaultRevision {
			return false, nil
		}
	}
	r.logger.Info("start etcd existing")
	if err := r.etcdRunner.StartExisting(r.envConfig.WriteEnv()); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Runner) loop(ctx context.Context, start chan struct{}) error {
	for {
		peerClientCtx, cancel := context.WithTimeout(ctx, r.checkQuorumTimeout)
		defer cancel()
		client, err := etcdutil.NewClientFromPeers(peerClientCtx, r.logger, r.envConfig.ClusterPeerURLs(), r.envConfig.PeerTLSConfig, r.envConfig.ClientTLSConfig)
		if err == nil {
			defer client.Close()
			err = client.CheckQuorum(peerClientCtx)
			if err == nil {
				addQuorumMemberCtx, cancel := context.WithTimeout(ctx, r.addQuorumMemberTimeout)
				defer cancel()
				if err := r.addMemberToQuorum(addQuorumMemberCtx, client); err != nil {
					return err
				}
				return nil
			}
		}
		r.logger.Info("quorum not found")

		select {
		case <-start:
			return r.etcdRunner.StartNew(r.envConfig.WriteEnv())
		case <-ctx.Done():
			return nil
		}
	}
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
	return r.etcdRunner.StartExisting(r.envConfig.WriteEnv())
}

func (r *Runner) replaceMember(ctx context.Context, client etcdutil.EtcdClient) error {
	listResp, err := client.MemberList(ctx)
	if err != nil {
		return fmt.Errorf("list cluster members: %w", err)
	}
	localMember := findMemberInCluster(listResp, r.envConfig.Env["ETCD_NAME"], r.envConfig.InitialAdvertisePeerURLs)

	// replace my node to join cluster
	// if my node already exists, it needs to be replaced
	if localMember != nil && len(listResp.GetMembers()) >= len(r.envConfig.InitialCluster) {
		r.logger.Info("remove local member", zap.Uint64("ID", localMember.GetID()))

		listResp, err = client.MemberRemove(ctx, localMember.GetID())
		if err != nil {
			return fmt.Errorf("remove cluster member %d: %w", localMember.GetID(), err)
		}
		localMember = findMemberInCluster(listResp, r.envConfig.Env["ETCD_NAME"], r.envConfig.InitialAdvertisePeerURLs)
		r.logger.Info("removed local member")
	}

	if localMember == nil && len(listResp.GetMembers()) < len(r.envConfig.InitialCluster) {
		listResp, err = client.MemberAdd(ctx, r.envConfig.InitialAdvertisePeerURLs)
		if err != nil {
			return fmt.Errorf("add member: %w", err)
		}
		localMember = findMemberInCluster(listResp, r.envConfig.Env["ETCD_NAME"], r.envConfig.InitialAdvertisePeerURLs)
		r.logger.Info("added local member", zap.Uint64("ID", localMember.GetID()))
	}
	return nil
}

func (r *Runner) updateMembers(ctx context.Context, client etcdutil.EtcdClient) error {
	listResp, err := client.MemberList(ctx)
	if err != nil {
		return fmt.Errorf("list cluster members: %w", err)
	}

	for _, member := range listResp.GetMembers() {
		switch {
		case member.GetID() == 0:
		case member.GetName() == r.envConfig.Env["ETCD_NAME"], util.HasMatchingElement(member.GetPeerURLs(), r.envConfig.InitialAdvertisePeerURLs):
			if !slices.Equal(member.GetPeerURLs(), r.envConfig.InitialAdvertisePeerURLs) {
				listResp, err = client.MemberUpdate(ctx, member.GetID(), r.envConfig.InitialAdvertisePeerURLs)
				if err != nil {
					return err
				}
				r.logger.Info("update local member", zap.Uint64("ID", member.GetID()), zap.String("peerURLs", fmt.Sprintf("%+v", r.envConfig.InitialAdvertisePeerURLs)))
			}
			r.envConfig.InitialCluster[r.envConfig.Env["ETCD_NAME"]] = r.envConfig.InitialAdvertisePeerURLs

		case member.GetName() == "":
		default:
			memberPeerURLs, ok := r.envConfig.InitialCluster[member.GetName()]
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
				_, err := client.MemberUpdate(ctx, member.GetID(), newMemberPeerURLs)
				if err != nil {
					return err
				}
				r.envConfig.InitialCluster[member.GetName()] = newMemberPeerURLs
				r.logger.Info("update member", zap.Uint64("ID", member.GetID()), zap.String("peerURLs", fmt.Sprintf("%+v", newMemberPeerURLs)))
			}
		}
	}
	return nil
}

func (r *Runner) clearLocalData() error {
	if err := removeDir(r.envConfig.Env["ETCD_DATA_DIR"]); err != nil {
		return fmt.Errorf("remove data dir: %w", err)
	}
	if err := removeDir(r.envConfig.Env["ETCD_WAL_DIR"]); err != nil {
		return fmt.Errorf("remove data dir: %w", err)
	}
	r.logger.Info("cleaned out existing data")
	return nil
}

func (r *Runner) getLocalRevision() (int64, error) {
	revision, err := etcdutil.GetDataRevision(r.envConfig.Env["ETCD_DATA_DIR"], r.logger)
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
	ok, err := snapshot.VerifyBucket(ctx, r.s3Client)
	if err != nil {
		return defaultRevision, fmt.Errorf("verify backup bucket: %w", err)
	}
	if !ok {
		return defaultRevision, fmt.Errorf("verify backup bucket")
	}
	ok, err = snapshot.Restore(ctx, r.logger, r.s3Client, r.objectPrefix, r.envConfig.RestoreConfig("", r.restoreRevisionBump), r.SnapshotEncryptionKey)
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
