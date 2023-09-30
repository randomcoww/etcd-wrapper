package main

import (
	"github.com/randomcoww/etcd-wrapper/pkg/status"
	"log"
	"time"
)

type clusterState int

const (
	clusterStateNew          clusterState  = 0
	clusterStateExisting     clusterState  = 1
	clusterStateWait         clusterState  = 2
	clusterStateWaitExisting clusterState  = 3
	waitExistingCountAllowed int           = 10
	failureCountAllowed      int           = 10
	waitDuration             time.Duration = 6 * time.Second
)

func main() {
	v, err := status.New()
	if err != nil {
		panic(err)
	}

	var failureCount, waitExistingCount int
	var state clusterState = clusterStateNew

L:
	for {
		select {
		case <-time.After(waitDuration):

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

			if v.Healthy {
				state = clusterStateExisting
				failureCount = 0
				waitExistingCount = 0
			}

			switch state {
			case clusterStateWait:

			case clusterStateWaitExisting:
				waitExistingCount++
				log.Printf("Wait existing failed count: %v (of %v)", waitExistingCount, waitExistingCountAllowed)

				if waitExistingCount < waitExistingCountAllowed {
					continue L
				}

				log.Printf("Wait existing exceeded allowed count.")
				state = clusterStateNew
				waitExistingCount = 0
				continue L

			case clusterStateNew:
				log.Printf("Cluster not found. Writing manifest for restore or new cluster.")

				if err = v.WritePodManifest(true); err != nil {
					log.Fatalf("Failed to write pod manifest: %v", err)
				}
				state = clusterStateWait
				continue L

			case clusterStateExisting:
				if !v.Healthy {
					failureCount++
					log.Printf("Health check failed count: %v (of %v)", failureCount, failureCountAllowed)

					if failureCount < failureCountAllowed {
						continue L
					}

					log.Printf("Failure exceeded allowed count. Writing manifest for restore or new cluster.")
					if err = v.WritePodManifest(true); err != nil {
						log.Fatalf("Failed to write pod manifest: %v", err)
					}
					state = clusterStateWait
					failureCount = 0
					continue L
				}

				log.Printf("Cluster %v found.", *v.ClusterID)

				if v.MemberSelf.MemberID == nil || *v.MemberSelf.MemberIDFromCluster != *v.MemberSelf.MemberID {
					log.Printf("Member mismatch with %v. Replacing member.", *v.MemberSelf.MemberIDFromCluster)
					if err = v.ReplaceMember(v.MemberSelf); err != nil {
						log.Fatalf("Failed to replace member. Writing manifest for restore or new cluster.")

						if err = v.WritePodManifest(true); err != nil {
							log.Fatalf("Failed to write pod manifest: %v", err)
						}
						state = clusterStateWait
						continue L
					}
					log.Printf("Replaced member with new %v. Writing manifest to join existing cluster.", *v.MemberSelf.MemberIDFromCluster)

					if err = v.WritePodManifest(false); err != nil {
						log.Fatalf("Failed to write pod manifest: %v", err)
					}
					state = clusterStateWaitExisting
					continue L
				}
			}
		}
	}
}
