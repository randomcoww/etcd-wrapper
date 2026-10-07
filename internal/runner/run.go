package runner

import (
	"context"
	"fmt"
	"os"
	"time"

	c "github.com/randomcoww/etcd-wrapper/config"
	"github.com/randomcoww/etcd-wrapper/internal/etcdutil"
	"github.com/randomcoww/etcd-wrapper/internal/s3util"
	"github.com/randomcoww/etcd-wrapper/internal/snapshot"
	"github.com/randomcoww/etcd-wrapper/internal/util"
	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.uber.org/zap"
)

type etcdProcess interface {
	StartNew([]string) error
	StartExisting([]string) error
	Stop() error
	Wait() error
}

type Runner struct {
	logger           *zap.Logger
	checkQuorumDelay time.Duration
	clientTimeout    time.Duration
	etcdRunner       etcdProcess
	envConfig        *c.EnvConfig
	s3Client         s3util.Client
	objectPrefix     string
}

func (r *Runner) runInterval(ctx context.Context, revisionBump uint64) error {
	client, err := etcdutil.NewClientFromPeers(ctx, r.logger, r.envConfig.ClusterPeerURLs, r.envConfig.PeerTLSConfig, r.envConfig.ClientTLSConfig)

	if err == nil {
		defer client.Close()

		c, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		err = client.CheckQuorum(c)
	}
	if err != nil {
		r.logger.Info("quorum not found")
		//
		// get current data revision. If invalid, delete data and return revision 0
		//
		revision, err := r.getLocalRevision()
		if err != nil {
			return fmt.Errorf("get cluster revision: %w", err)
		}
		if revision == 0 {
			revision, err = r.restoreSnapshot(ctx)
			if err != nil {
				return fmt.Errorf("get cluster revision: %w", err)
			}
			if revision == 0 {
				r.logger.Info("start etcd new")
				return r.etcdRunner.StartNew(r.envConfig.WriteEnv())
			}
		}
		r.logger.Info("start etcd existing")
		return r.etcdRunner.StartExisting(r.envConfig.WriteEnv())
	}
	r.logger.Info("existing quorum found")

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
	if localMember != nil && len(listResp.GetMembers()) >= len(r.envConfig.ClusterPeerURLs) {
		r.logger.Info("remove local member", zap.Uint64("ID", localMember.GetID()))

		listResp, err = client.MemberRemove(ctx, localMember.GetID())
		if err != nil {
			return fmt.Errorf("remove cluster member %d: %w", localMember.GetID(), err)
		}
		localMember = findMemberInCluster(listResp, r.envConfig.Env["ETCD_NAME"], r.envConfig.InitialAdvertisePeerURLs)
		r.logger.Info("removed local member")
	}

	if localMember == nil && len(listResp.GetMembers()) < len(r.envConfig.ClusterPeerURLs) {
		listResp, err = client.MemberAdd(ctx, r.envConfig.InitialAdvertisePeerURLs)
		if err != nil {
			return fmt.Errorf("add member: %w", err)
		}
		localMember = findMemberInCluster(listResp, r.envConfig.Env["ETCD_NAME"], r.envConfig.InitialAdvertisePeerURLs)
		r.logger.Info("added local member", zap.Uint64("ID", localMember.GetID()))
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
			return 0, fmt.Errorf("clear local data: %w", err)
		}
		r.logger.Info("cleared local data")
	}
	r.logger.Info("local data", zap.Int64("revision", revision))
	return revision, nil
}

func (r *Runner) restoreSnapshot(ctx context.Context) (int64, error) {
	if err := r.clearLocalData(); err != nil {
		return 0, err
	}
	ok, err := snapshot.VerifyBucket(ctx, r.s3Client)
	if err != nil {
		return 0, fmt.Errorf("verify backup bucket: %w", err)
	}
	if !ok {
		return 0, fmt.Errorf("verify backup bucket")
	}
	ok, err = snapshot.Restore(ctx, r.logger, r.s3Client, r.objectPrefix, r.envConfig.RestoreConfig("", 10000))
	if err != nil {
		return 0, fmt.Errorf("restore snapshot: %w", err)
	}
	if ok {
		return r.getLocalRevision()
	}
	return 0, nil
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
