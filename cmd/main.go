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
		log.Printf("********** Waiting %v **********", waitDuration)
		time.Sleep(waitDuration)

		err = v.SyncStatus()
		if err != nil {
			log.Printf("Cluster status update failed: %v", err)
			waitDuration = 6 * time.Second
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

		if v.ClusterID == nil {
			log.Printf("Cluster not found. Writing pod manifest for restore or new cluster.")

			err = v.WritePodManifest(true)
			if err != nil {
				log.Fatalf("Failed to write pod manifest: %v", err)
			}
			waitDuration = 1 * time.Minute
			continue
		}

		if !v.Healthy {
			log.Printf("Cluster unhealthy. Writing manifest for restore or new cluster.")

			err = v.WritePodManifest(true)
			if err != nil {
				log.Fatalf("Failed to write pod manifest: %v", err)
			}
			waitDuration = 1 * time.Minute
			continue
		}

		if v.MemberSelf.MemberIDFromCluster == nil || v.MemberSelf.MemberID == nil ||
			*v.MemberSelf.MemberIDFromCluster != *v.MemberSelf.MemberID {

			log.Printf("Cluster %v found. Joining existing cluster.", *v.ClusterID)

			err = v.ReplaceMember(v.MemberSelf)
			if err != nil {
				log.Fatalf("Failed to replace existing member. Writing manifest for restore or new cluster.")

				err = v.WritePodManifest(true)
				if err != nil {
					log.Fatalf("Failed to write pod manifest: %v", err)
				}
				waitDuration = 1 * time.Minute
				continue
			}
			log.Printf("Replaced member with new ID %v. Writing manifest to join existing cluster %v.", *v.MemberSelf.MemberIDFromCluster, *v.ClusterID)

			err = v.WritePodManifest(false)
			if err != nil {
				log.Fatalf("Failed to write pod manifest: %v", err)
			}
			waitDuration = 1 * time.Minute
			continue
		}

		waitDuration = 6 * time.Second
	}
}
