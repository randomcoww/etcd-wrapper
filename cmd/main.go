package main

import (
	"github.com/randomcoww/etcd-wrapper/pkg/status"
	"log"
	"time"
)

type clusterState int

const (
	clusterStateNew             clusterState  = 0
	clusterStateHealthy         clusterState  = 1
	clusterStateWait            clusterState  = 2
	healthcheckFailCountAllowed int           = 16
	intervalDuration            time.Duration = 6 * time.Second
)

func main() {
	v, err := status.New()
	if err != nil {
		panic(err)
	}

	var healthcheckFailCount int
	var state clusterState = clusterStateNew

L:
	for {
		select {
		case <-time.After(intervalDuration):

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
				switch {
				case v.MemberSelf.Healthy, v.Healthy:
					log.Printf("Cluster healthy")
					state = clusterStateHealthy
					continue L

				default:
					log.Printf("Cluster not found. Writing manifest for restore or new cluster")
					if err = v.WritePodManifest(true); err != nil {
						log.Fatalf("Failed to write pod manifest: %v", err)
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
						}
						state = clusterStateWait
						continue L
					}
					log.Printf("Replaced member with new %v. Writing manifest to join existing cluster", *v.MemberSelf.MemberIDFromCluster)

					if err = v.WritePodManifest(false); err != nil {
						log.Fatalf("Failed to write pod manifest: %v", err)
					}
					state = clusterStateWait
					continue L

				default:
					healthcheckFailCount++
					log.Printf("Health check failed count: %v (of %v)", healthcheckFailCount, healthcheckFailCountAllowed)

					if healthcheckFailCount < healthcheckFailCountAllowed {
						continue L
					}
					state = clusterStateNew
					continue L
				}

			case clusterStateWait:
				switch {
				case v.MemberSelf.Healthy:
					log.Printf("Cluster healthy")
					state = clusterStateHealthy
					continue L

				default:
					log.Printf("Waiting health check pass")
					continue L
				}
			}
		}
	}
}
