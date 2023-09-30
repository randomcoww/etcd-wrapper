package main

import (
	"github.com/randomcoww/etcd-wrapper/pkg/status"
	"log"
	"time"
)

func main() {
	v, err := status.New()
	if err != nil {
		panic(err)
	}

	waitDuration := 6 * time.Second
	for {
		log.Printf("wait %v", waitDuration)
		time.Sleep(waitDuration)

		err = v.SyncStatus()
		if err != nil {
			log.Printf("status error: %+v", err)
			waitDuration = 6 * time.Second
			continue
		}

		// log.Printf("status: %+v\n", v)
		if v.ClusterID != nil {
			log.Printf("ClusterID: %v\n", *v.ClusterID)
		}
		if v.LeaderID != nil {
			log.Printf("LeaderID: %v\n", *v.LeaderID)
		}
		log.Printf("Healthy: %v\n", v.Healthy)

		for _, member := range v.MembersHealthy {
			log.Printf("Member: %v %v\n", member.Name, *member.ClusterID)

			if member.MemberIDFromCluster != nil {
				log.Printf("MemberIDFromCluster: %v\n", *member.MemberIDFromCluster)
			}
			if member.MemberID != nil {
				log.Printf("MemberID: %v\n", *member.MemberID)
			}
		}

		if v.ClusterID == nil {
			log.Printf("No cluster found. Write manifest for new cluster")

			err = v.WritePodManifest(true)
			if err != nil {
				log.Fatalf("failed to write pod manifest: %v", err)
			}
			waitDuration = 1 * time.Minute
			continue
		}

		if !v.Healthy {
			log.Printf("Cluster unhealthy. Write manifest for new cluster")

			err = v.WritePodManifest(true)
			if err != nil {
				log.Fatalf("failed to write pod manifest: %v", err)
			}
			waitDuration = 1 * time.Minute
			continue
		}

		if v.MemberSelf.MemberIDFromCluster == nil || v.MemberSelf.MemberID == nil ||
			*v.MemberSelf.MemberIDFromCluster != *v.MemberSelf.MemberID {

			log.Printf("Join existing cluster.")

			err = v.ReplaceMember(v.MemberSelf)
			log.Printf("Replacing existing member: %v", v.MemberSelf.MemberIDFromCluster)
			if err != nil {
				log.Fatalf("Failed to replace existing member.")

				err = v.WritePodManifest(true)
				if err != nil {
					log.Fatalf("failed to write pod manifest: %v", err)
				}
				waitDuration = 1 * time.Minute
				continue
			}
			log.Printf("Got new member ID: %v", v.MemberSelf.MemberIDFromCluster)
			err = v.WritePodManifest(false)
			if err != nil {
				log.Fatalf("failed to write pod manifest: %v", err)
			}
			waitDuration = 1 * time.Minute
			continue
		}

		waitDuration = 6 * time.Second
	}
}
