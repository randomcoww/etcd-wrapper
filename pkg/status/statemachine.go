package status

import (
	"fmt"
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/etcdutil"
	"log"
)

type MemberState int

const (
	MemberStateInit    MemberState = 0
	MemberStateWait    MemberState = 1
	MemberStateHealthy MemberState = 2
	MemberStateFailed  MemberState = 3
)

func (v *Status) Run(args *arg.Args) error {
	defer v.EtcdPod.DeleteFile(args)
	var healthCheckFailedCount, readyCheckFailedCount, memberCheckFailedCount int

	replaceMember := func(m etcdutil.Member) error {
		if err := v.ReplaceMember(m); err != nil {
			log.Printf("Failed to replace member: %v", err)
			return err
		}
		log.Printf("Replaced member %v", m.GetID())
		return nil
	}

	replaceMemberSelf := func(m etcdutil.Member) error {
		if err := replaceMember(m); err != nil {
			return err
		}
		args.ListenPeerURLs = m.GetPeerURLs()
		args.InitialAdvertisePeerURLs = m.GetPeerURLs()
		return nil
	}

	createPodForNewCluster := func() error {
		log.Printf("Creating new node")
		args.InitialClusterState = "new"
		err := v.EtcdPod.WriteFile(args)
		if err != nil {
			log.Printf("Failed to write pod manifest file: %v", err)
			return err
		}
		return nil
	}

	createPodForExistingCluster := func() error {
		log.Printf("Attempt to join existing cluster")
		args.InitialClusterState = "existing"
		err := v.EtcdPod.WriteFile(args)
		if err != nil {
			log.Printf("Failed to write pod manifest file: %v", err)
			return err
		}
		return nil
	}

	// main

	go func() {
		for {
			select {
			case t := <-v.HealthCheckTick:
				v.HealthCheckChan <- t

			case t := <-v.BackupTick:
				v.BackupChan <- t
			}
		}
	}()

	for {
		select {
		case <-v.Quit:
			return nil

		case <-v.BackupChan:
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

		case <-v.HealthCheckChan:
			if err := v.SyncStatus(args); err != nil {
				return err
			}
			switch v.MemberState {
			case MemberStateInit:
				switch {
				case v.Self.IsHealthy():
					v.MemberState = MemberStateHealthy
					log.Printf("State transitioned to healhty")

				case v.Healthy:
					if memberToReplace := v.GetMemberToReplace(); memberToReplace != nil {
						replaceMemberSelf(memberToReplace)
					}
					fallthrough

				default:
					if err := createPodForNewCluster(); err != nil {
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
					log.Printf("Health check failed %v of %v", healthCheckFailedCount, args.HealthCheckFailedCountMax)
					if healthCheckFailedCount >= args.HealthCheckFailedCountMax {
						healthCheckFailedCount = 0
						v.MemberState = MemberStateFailed
						log.Printf("State transitioned to failed")
					}

				case v.Self == v.Leader && memberToReplace != nil:
					healthCheckFailedCount = 0

					memberCheckFailedCount++
					log.Printf("Unresponsive member check %v of %v", memberCheckFailedCount, args.HealthCheckFailedCountMax)
					if memberCheckFailedCount >= args.HealthCheckFailedCountMax {
						memberCheckFailedCount = 0
						replaceMember(memberToReplace)
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
						replaceMemberSelf(memberToReplace)
					}
					if err := createPodForExistingCluster(); err != nil {
						return err
					}
					v.MemberState = MemberStateHealthy
					log.Printf("State transitioned to healthy")

				default:
					if err := createPodForNewCluster(); err != nil {
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
