package main

import (
	"github.com/randomcoww/etcd-wrapper/pkg/status"
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
	v, err := status.New()
	if err != nil {
		panic(err)
	}

	var healthcheckFailCount int
	var readinessFailCount int
	var state clusterState = clusterStateHealthy

	intervalTick := time.NewTicker(v.HealthCheckInterval)
	backupIntervalTick := time.NewTicker(v.BackupInterval)

L:
	for {
		select {
		case <-backupIntervalTick.C:

			err = v.SyncStatus()
			if err != nil {
				log.Printf("Cluster status update failed: %v", err)
				continue
			}

			if !v.MemberSelf.Healthy ||
				*v.MemberSelf.MemberID != *v.BackupMemberID {

				log.Printf("Member not selected for backup")
				continue
			}
			log.Printf("Member selected for backup. Starting snapshot backup")
			if err := v.BackupSnapshot(); err != nil {
				log.Fatalf("Snapshot backup failed: %v", err)
				continue
			}
			log.Printf("Snapshot backup success")
			continue

		case <-intervalTick.C:

			err = v.SyncStatus()
			if err != nil {
				log.Printf("Cluster status update failed: %v", err)
				continue
			}

			statusYaml, err := v.ToYaml()
			if err != nil {
				log.Fatalf("Failed to parse cluster status: %v", err)
			}
			log.Printf("Cluster:\n%s", statusYaml)

			for _, m := range v.Members {
				statusYaml, err = m.ToYaml()
				if err != nil {
					log.Fatalf("Failed to parse member: %v", err)
				}
				log.Printf("Member %s:\n%s", m.Name, statusYaml)
			}

			switch state {

			case clusterStateNew:
				healthcheckFailCount = 0
				readinessFailCount = 0

				switch {
				case v.MemberSelf.Healthy, v.Healthy:
					log.Printf("Cluster entered healthy state")
					state = clusterStateHealthy
					continue L

				default:
					log.Printf("Cluster not found. Writing manifest for restore or new cluster")
					if err = v.WritePodManifest(true); err != nil {
						log.Fatalf("Failed to write pod manifest: %v", err)
						panic(err)
					}
					state = clusterStateWait
					continue L
				}

			case clusterStateHealthy:
				switch {
				case v.MemberSelf.Healthy:
					log.Printf("Cluster healthy")
					healthcheckFailCount = 0
					continue L

				case v.Healthy:
					log.Printf("Member mismatch with %v. Replacing member.", *v.MemberSelf.MemberIDFromCluster)
					healthcheckFailCount = 0
					if err = v.ReplaceMember(v.MemberSelf); err != nil {
						log.Fatalf("Failed to replace member. Writing manifest for restore or new cluster")

						if err = v.WritePodManifest(true); err != nil {
							log.Fatalf("Failed to write pod manifest: %v", err)
							panic(err)
						}
						state = clusterStateWait
						continue L
					}
					log.Printf("Replaced member with new %v. Writing manifest to join existing cluster", *v.MemberSelf.MemberIDFromCluster)

					if err = v.WritePodManifest(false); err != nil {
						log.Fatalf("Failed to write pod manifest: %v", err)
						panic(err)
					}
					state = clusterStateWait
					continue L

				default:
					healthcheckFailCount++
					log.Printf("Health check failed count: %v (of %v)", healthcheckFailCount, v.HealthCheckFailCountAllowed)

					if healthcheckFailCount < v.HealthCheckFailCountAllowed {
						continue L
					}
					state = clusterStateNew
					continue L
				}

			case clusterStateWait:
				switch {
				case v.MemberSelf.Healthy:
					log.Printf("Cluster entered healthy state")
					state = clusterStateHealthy
					readinessFailCount = 0
					continue L

				default:
					readinessFailCount++
					log.Printf("Readiness check failed count: %v (of %v)", readinessFailCount, v.ReadinessFailCountAllowed)

					if readinessFailCount < v.ReadinessFailCountAllowed {
						continue L
					}
					state = clusterStateNew
					continue L
				}
			}
		}
	}
}
