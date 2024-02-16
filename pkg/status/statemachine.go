package status

import (
	"bytes"
	"encoding/json"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/podspec"
	"github.com/randomcoww/etcd-wrapper/pkg/util"
	"io"
	"k8s.io/api/core/v1"
	"log"
	"fmt"
	"context"
	"time"
)

type clusterState int

const (
	clusterStateWait    clusterState = 0
	clusterStateHealthy clusterState = 1
	clusterStateFailed  clusterState = 2
)

func (v *Status) Run(args *arg.Args) error {
	var healthCheckFailedCount int
	var readyCheckFailedCount int
	var state clusterState = clusterStateHealthy

	intervalTick := time.NewTicker(args.HealthCheckInterval)
	backupIntervalTick := time.NewTicker(args.BackupInterval)

	for {
		select {
		case <-backupIntervalTick.C:

			if err := v.SyncStatus(args); err != nil {
				return err
			}
			if !v.Healthy {
				continue
			}
			if v.Self == nil {
				continue
			}
			if !v.Self.Healthy() {
				continue
			}
			if err := v.Defragment(args); err != nil {
				continue
			}
			if v.Self == v.Leader {
				log.Printf("Member is leader, starting snapshot backup.")
				if err := v.SnapshotBackup(args); err != nil {
					log.Fatalf("Snapshot backup failed: %v", err)
				} else {
					log.Printf("Snapshot backup success.")
				}
			}

		case <-intervalTick.C:

			if err := v.SyncStatus(args); err != nil {
				return err
			}

			switch state {
			case clusterStateWait:
				switch {
				case v.Self != nil && v.Self.Healthy():
					state = clusterStateHealthy

				default:
					readyCheckFailedCount++
					if readyCheckFailedCount < args.ReadyCheckFailedCountMax {
						readyCheckFailedCount = 0
						if err := deletePodManifest(args); err != nil {
							log.Fatalf("Failed to clean up pod manifest: %v", err)
						}
						return fmt.Errorf("Failed ready check after member update. Quitting.")
					}
				}

			case clusterStateHealthy:
				switch {
				case v.Self != nil && v.Self.Healthy():
					// promote local member from learner if it is a learner
					if v.Self.Status.GetRaftIndex() >= v.Leader.Status.GetRaftIndex() {
						log.Printf("Promoting learning member.")

						if err := v.PromoteMember(args); err != nil {
							log.Fatalf("Failed to promote member: %v", err)
						} else {
							log.Printf("Promoted learning member.")
						}
					}

				default:
					healthCheckFailedCount++
					if healthCheckFailedCount < args.HealthCheckFailedCountMax {
						healthCheckFailedCount = 0
						state = clusterStateFailed
						log.Fatalf("Member failed health check, transitioning to failed.")
					}
				}

			case clusterStateFailed:
				switch {
				case v.Self != nil && v.Self.Healthy():
					state = clusterStateHealthy
					log.Printf("Member healthy. Transitioning to healthy.")

				case v.Self != nil:
					// local member has the wrong member ID? try replacing
					log.Printf("Member ID mismatch. Replacing member.")

					if err := v.ReplaceMember(args); err != nil {
						log.Fatalf("Replace member failed. Restarting node: %v", err)

						if err = writePodManifest(args); err != nil {
							log.Fatalf("Failed to write pod manifest for new node, %v", err)
							return err
						}
					}
					state = clusterStateWait

				case v.Healthy:
					// local member doesn't exist
					// list members failing? start local member
					log.Printf("Member not found. Creating new node.")

					if err := writePodManifest(args); err != nil {
						log.Fatalf("Failed to write pod manifest for new node, %v", err)
						return err
					}
					state = clusterStateWait

				default:
					// cluster not found
					log.Printf("Cluster not found. Creating new node.")

					if err := writePodManifest(args); err != nil {
						log.Fatalf("Failed to write pod manifest for new node, %v", err)
						return err
					}
					state = clusterStateWait
				}
			}

			statusYaml, err := v.ToYaml()
			if err != nil {
				log.Fatalf("Failed to parse cluster status: %v", err)
			}
			log.Printf("Cluster status:\n%s", statusYaml)
		}
	}
}

func writePodManifest(args *arg.Args) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var pod *v1.Pod
	manifestVersion := fmt.Sprintf("%v", time.Now().Unix())

	ok, err := args.S3Client.Download(ctx, args.S3BackupBucket, args.S3BackupKey, func(ctx context.Context, r io.Reader) error {
		return util.WriteFile(r, args.EtcdSnapshotFile)
	})
	if err != nil {
		return fmt.Errorf("Error getting snapshot: %v", err)
	}
	if !ok {
		log.Printf("Snapshot not found. Starting new cluster")
		args.InitialClusterState = "new"
		pod = podspec.Create(args, false, manifestVersion)

	} else {
		log.Printf("Successfully got snapshot. Restoring cluster")
		args.InitialClusterState = "existing"
		pod = podspec.Create(args, true, manifestVersion)
	}

	manifest, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteFile(io.NopCloser(bytes.NewReader(manifest)), args.EtcdPodManifestFile)
}

func deletePodManifest(args *arg.Args) error {
	return util.DeleteFile(args.EtcdPodManifestFile)
}
