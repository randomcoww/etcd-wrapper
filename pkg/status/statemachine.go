package status

import (
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
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

func (v *Status) Run(args *arg.Args, tickCountMax int) error {
	defer v.EtcdPod.DeleteFile(args)

	var healthCheckFailedCount, readyCheckFailedCount, memberCheckFailedCount, tickCount int
	intervalTick := time.NewTicker(args.HealthCheckInterval)
	backupIntervalTick := time.NewTicker(args.BackupInterval)

	for {
		select {
		case <-v.quit:
			return nil

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
			tickCount++
			if tickCountMax > 0 && tickCount >= tickCountMax {
				tickCount = 0
				v.quit <- struct{}{}
			}

			switch v.MemberState {
			case MemberStateInit:
				switch {
				case v.Self.IsHealthy():
					v.MemberState = MemberStateHealthy
					log.Printf("State transitioned to healhty")

				case v.Healthy:
					if memberToReplace := v.GetMemberToReplace(); memberToReplace != nil {
						if err := v.ReplaceMember(memberToReplace); err != nil {
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
					if err := v.EtcdPod.WriteFile(args); err != nil {
						log.Printf("Failed to write pod manifest for new node, %v", err)
						return err
					}
					v.MemberState = MemberStateHealthy
					log.Printf("State transitioned to wait healthy")
				}

			case MemberStateWait:
				switch {
				case v.Self.IsHealthy():
					readyCheckFailedCount = 0

					v.MemberState = MemberStateHealthy
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
						v.MemberState = MemberStateFailed
						log.Printf("State transitioned to failed")
					}

				case memberToReplace != nil:
					memberCheckFailedCount++
					log.Printf("Unresponsive member check %v of %v", memberCheckFailedCount, args.HealthCheckFailedCountMax)
					if memberCheckFailedCount >= args.HealthCheckFailedCountMax {
						memberCheckFailedCount = 0
						if err := v.ReplaceMember(memberToReplace); err != nil {
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
					v.MemberState = MemberStateHealthy
					log.Printf("State transitioned to healhty")

				case v.Healthy:
					if memberToReplace := v.GetMemberToReplace(); memberToReplace != nil {
						if err := v.ReplaceMember(memberToReplace); err != nil {
							log.Printf("Replace member failed")
						} else {
							log.Printf("Replaced member: %v", memberToReplace.GetID())
							args.ListenPeerURLs = memberToReplace.GetPeerURLs()
							args.InitialAdvertisePeerURLs = memberToReplace.GetPeerURLs()
						}
					}

					log.Printf("Attempt to join existing cluster")
					args.InitialClusterState = "existing"
					if err := v.EtcdPod.WriteFile(args); err != nil {
						log.Printf("Failed to write pod manifest for new node, %v", err)
						return err
					}
					v.MemberState = MemberStateHealthy
					log.Printf("State transitioned to healthy")

				default:
					log.Printf("Creating new node")
					args.InitialClusterState = "new"
					if err := v.EtcdPod.WriteFile(args); err != nil {
						log.Printf("Failed to write pod manifest for new node, %v", err)
						return err
					}
					v.MemberState = MemberStateWait
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
