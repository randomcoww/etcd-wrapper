package main

import (
	"github.com/randomcoww/etcd-wrapper/pkg/arg"
	"github.com/randomcoww/etcd-wrapper/pkg/status"
	"github.com/randomcoww/etcd-wrapper/pkg/util/etcdutil"
	"log"
	"time"
)

type clusterState int

const (
	clusterStateNew     clusterState = 0
	clusterStateHealthy clusterState = 1
	clusterStateWait    clusterState = 2
)

func main() {
	args, err := arg.New()
	if err != nil {
		panic(err)
	}

	status, err := status.New(args)
	if err != nil {
		panic(err)
	}

	var healthcheckFailCount int
	var readinessFailCount int
	var state clusterState = clusterStateHealthy
	var etcd etcdutil.StatusCheck

	intervalTick := time.NewTicker(args.HealthCheckInterval)
	backupIntervalTick := time.NewTicker(args.BackupInterval)

L:
	for {
		select {
		case <-backupIntervalTick.C:

			err = status.UpdateFromStatus(args, etcd)
			if err != nil {
				log.Printf("Cluster status update failed: %v", err)
				continue
			}
			if !status.MemberSelf.Healthy {
				log.Printf("Member unhealthy. Not performing backup")
				continue
			}

			if err := status.Defragment(args); err != nil {
				log.Fatalf("Member defragment failed: %v", err)
				continue
			}
			log.Printf("Defragment success")
			if *status.MemberSelf.MemberID != *status.BackupMemberID {
				log.Printf("Member not selected for backup")
				continue
			}
			log.Printf("Member selected for backup. Starting snapshot backup")
			if err := status.BackupSnapshot(args); err != nil {
				log.Fatalf("Snapshot backup failed: %v", err)
				continue
			}
			log.Printf("Snapshot backup success")
			continue

		case <-intervalTick.C:

			err = status.UpdateFromStatus(args, etcd)
			if err != nil {
				log.Fatalf("Cluster status update failed: %v", err)
			}
			status.SetMembersHealth()

			statusYaml, err := status.ToYaml()
			if err != nil {
				log.Fatalf("Failed to parse cluster status: %v", err)
			}
			log.Printf("Cluster status:\n%s", statusYaml)

			switch state {
			case clusterStateNew:
				healthcheckFailCount = 0
				readinessFailCount = 0

				switch {
				case status.MemberSelf.Healthy, status.Healthy:
					state = clusterStateHealthy
					continue L

				default:
					log.Printf("Cluster not found. Writing manifest for restored or new cluster")
					if err = status.WritePodManifest(args, true); err != nil {
						log.Fatalf("Failed to write pod manifest: %v", err)
						panic(err)
					}
					state = clusterStateWait
					continue L
				}

			case clusterStateHealthy:
				switch {
				case status.MemberSelf.Healthy:
					healthcheckFailCount = 0
					continue L

				case status.Healthy:
					log.Printf("Member ID mismatch. Replacing member")
					healthcheckFailCount = 0
					if err = status.ReplaceMember(args, status.MemberSelf); err != nil {
						log.Fatalf("Failed to replace member. Writing manifest for restore or new cluster")

						if err = status.WritePodManifest(args, true); err != nil {
							log.Fatalf("Failed to write pod manifest: %v", err)
							panic(err)
						}
						state = clusterStateWait
						continue L
					}
					log.Printf("Replaced member with new %v. Writing manifest to join existing cluster", *status.MemberSelf.MemberIDFromCluster)

					if err = status.WritePodManifest(args, false); err != nil {
						log.Fatalf("Failed to write pod manifest: %v", err)
						panic(err)
					}
					state = clusterStateWait
					continue L

				default:
					healthcheckFailCount++
					log.Printf("Health check failed count: %v (of %v)", healthcheckFailCount, args.HealthCheckFailCountAllowed)

					if healthcheckFailCount < args.HealthCheckFailCountAllowed {
						continue L
					}
					state = clusterStateNew
					continue L
				}

			case clusterStateWait:
				switch {
				case status.MemberSelf.Healthy:
					state = clusterStateHealthy
					readinessFailCount = 0
					continue L

				default:
					readinessFailCount++
					log.Printf("Readiness check failed count: %v (of %v)", readinessFailCount, args.ReadinessFailCountAllowed)

					if readinessFailCount < args.ReadinessFailCountAllowed {
						continue L
					}
					err := status.DeletePodManifest(args)
					if err != nil {
						log.Fatalf("Failed to clean up etcd pod manifest file: %v", err)
					}
					panic("Quitting with readiness failed")
				}
			}
		}
	}
}
