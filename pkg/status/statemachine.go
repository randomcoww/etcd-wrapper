package status

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
	"time"
)

type MemberState int

const (
	MemberStateInit    MemberState = 0
	MemberStateWait    MemberState = 1
	MemberStateHealthy MemberState = 2
	MemberStateFailed  MemberState = 3
)

func (v *Status) Run(args *arg.Args) error {
	defer deletePodManifest(args)

	var state MemberState = MemberStateInit
	var healthCheckFailedCount, readyCheckFailedCount, memberCheckFailedCount int
	intervalTick := time.NewTicker(args.HealthCheckInterval)
	backupIntervalTick := time.NewTicker(args.BackupInterval)

	for {
		log.Printf("State: %v", state)

		select {
		case <-backupIntervalTick.C:
			if err := v.SyncStatus(args); err != nil {
				return err
			}
			if !v.Self.IsHealthy() {
				continue
			}
			if err := v.Defragment(args); err != nil {
				log.Printf("Defragment error: %v", err)
				continue
			}
			log.Printf("Defragmented node")
			if v.Self != v.Leader {
				continue
			}

			log.Printf("Member is leader, starting snapshot backup.")
			if err := v.SnapshotBackup(args); err != nil {
				log.Printf("Snapshot backup failed: %v", err)
			} else {
				log.Printf("Snapshot backup success.")
			}

		case <-intervalTick.C:
			if err := v.SyncStatus(args); err != nil {
				return err
			}

			switch state {
			case MemberStateInit:
				switch {
				case v.Self.IsHealthy():
					state = MemberStateHealthy
					log.Printf("State transitioned to healhty")

				case v.Healthy:
					if memberToReplace := v.GetMemberToReplace(); memberToReplace != nil {
						if err := v.ReplaceMember(memberToReplace, args); err != nil {
							log.Printf("Replace member failed")
						} else {
							log.Printf("Replaced member: %v", memberToReplace.GetID())
							args.ListenPeerURLs = memberToReplace.GetPeerURLs()
							args.InitialAdvertisePeerURLs = memberToReplace.GetPeerURLs()
						}
					}
					fallthrough

				default:
					log.Printf("Attempt to join existing cluster")
					args.InitialClusterState = "existing"
					if err := writePodManifest(args); err != nil {
						log.Printf("Failed to write pod manifest for new node, %v", err)
						return err
					}
					state = MemberStateHealthy
					log.Printf("State transitioned to wait healthy")
				}

			case MemberStateWait:
				switch {
				case v.Self.IsHealthy():
					readyCheckFailedCount = 0

					state = MemberStateHealthy
					log.Printf("State transitioned to healhty")

				default:
					readyCheckFailedCount++
					log.Printf("Ready check failed %v of %v", readyCheckFailedCount, args.ReadyCheckFailedCountMax)
					if readyCheckFailedCount >= args.ReadyCheckFailedCountMax {
						readyCheckFailedCount = 0
						return fmt.Errorf("Failed ready check")
					}
				}

			case MemberStateHealthy:
				memberToReplace := v.GetMemberToReplace()

				switch {
				case !v.Self.IsHealthy():
					memberCheckFailedCount = 0

					healthCheckFailedCount++
					log.Printf("Health check %v of %v", healthCheckFailedCount, args.HealthCheckFailedCountMax)
					if healthCheckFailedCount >= args.HealthCheckFailedCountMax {
						memberCheckFailedCount = 0
						state = MemberStateFailed
						log.Printf("State transitioned to failed")
					}

				case memberToReplace != nil:
					memberCheckFailedCount++
					log.Printf("Unresponsive member check %v of %v", memberCheckFailedCount, args.HealthCheckFailedCountMax)
					if memberCheckFailedCount >= args.HealthCheckFailedCountMax {
						memberCheckFailedCount = 0
						if err := v.ReplaceMember(memberToReplace, args); err != nil {
							log.Printf("Replace member failed")
						}
						log.Printf("Replaced member: %v", memberToReplace.GetID())
					}

				default:
					memberCheckFailedCount = 0
					healthCheckFailedCount = 0
				}

			case MemberStateFailed:
				switch {
				case v.Self.IsHealthy():
					state = MemberStateHealthy
					log.Printf("State transitioned to healhty")

				case v.Healthy:
					if memberToReplace := v.GetMemberToReplace(); memberToReplace != nil {
						if err := v.ReplaceMember(memberToReplace, args); err != nil {
							log.Printf("Replace member failed")
						} else {
							log.Printf("Replaced member: %v", memberToReplace.GetID())
							args.ListenPeerURLs = memberToReplace.GetPeerURLs()
							args.InitialAdvertisePeerURLs = memberToReplace.GetPeerURLs()
						}
					}

					log.Printf("Attempt to join existing cluster")
					args.InitialClusterState = "existing"
					if err := writePodManifest(args); err != nil {
						log.Printf("Failed to write pod manifest for new node, %v", err)
						return err
					}
					state = MemberStateHealthy
					log.Printf("State transitioned to healthy")

				default:
					log.Printf("Creating new node")
					args.InitialClusterState = "new"
					if err := writePodManifest(args); err != nil {
						log.Printf("Failed to write pod manifest for new node, %v", err)
						return err
					}
					state = MemberStateWait
					log.Printf("State transitioned to wait")
				}
			}

			statusYaml, err := v.ToYaml()
			if err != nil {
				log.Printf("Failed to parse cluster status: %v", err)
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

	switch args.InitialClusterState {
	case "new":
		ok, err := args.S3Client.Download(ctx, args.S3BackupBucket, args.S3BackupKey, func(ctx context.Context, r io.Reader) error {
			return util.WriteFile(r, args.EtcdSnapshotFile)
		})
		if err != nil {
			return fmt.Errorf("Error getting snapshot: %v", err)
		}
		if !ok {
			log.Printf("Snapshot not found. Joining existing cluster")
			pod = podspec.Create(args, false, manifestVersion)

		} else {
			log.Printf("Successfully got snapshot. Restoring existing cluster")
			pod = podspec.Create(args, true, manifestVersion)
		}

	case "existing":
		pod = podspec.Create(args, false, manifestVersion)

	default:
		return fmt.Errorf("InitialClusterState not defined")
	}

	manifest, err := json.MarshalIndent(pod, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteFile(io.NopCloser(bytes.NewReader(manifest)), args.EtcdPodManifestFile)
}

func deletePodManifest(args *arg.Args) {
	util.DeleteFile(args.EtcdPodManifestFile)
}
