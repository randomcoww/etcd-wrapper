package main

import (
	"github.com/randomcoww/etcd-wrapper/pkg/status"
	"time"
	"log"
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
			panic(err)
		}

		log.Printf("status: %+v", v)

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

		if v.ClusterID != v.MemberSelf.ClusterID {
			log.Printf("Possible split brain. Write manifest for new cluster")

			// do add remove
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

		if v.MemberSelf.MemberIDFromCluster != v.MemberSelf.MemberID {
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
		}

		waitDuration = 6 * time.Second
	}
}

// func main() {
// 	endpoints := []string{
// 		"127.0.0.1:40004",
// 		"127.0.0.1:40005",
// 		"127.0.0.1:40006",
// 	}

// 	status, err := Status(endpoints, nil)
// 	fmt.Printf("status: %+v\nerr: %v\n", status, err)

// 	list, err := ListMembers(endpoints, nil)
// 	fmt.Printf("list: %+v\nerr: %v\n", list, err)
// }

// func newClient(ctx context.Context, endpoints []string, tlsConfig *tls.Config) (*clientv3.Client, error) {
// 	return clientv3.New(clientv3.Config{
// 		Endpoints:   endpoints,
// 		DialTimeout: 30 * time.Second,
// 		TLS:         tlsConfig,
// 		Context:     ctx,
// 	})
// }

// func Status(endpoints []string, tlsConfig *tls.Config) (resp *clientv3.StatusResponse, err error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// 	client, err := newClient(ctx, endpoints, tlsConfig)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer client.Close()

// 	errCh := make(chan error)
// 	respCh := make(chan *clientv3.StatusResponse)
// 	// defer close(errCh)
// 	// defer close(respCh)

// 	for i, endpoint := range endpoints {
// 		go func(i int, endpoint string) {
// 			resp, err = client.Status(ctx, endpoint)
// 			if err != nil {
// 				errCh <- err
// 				return
// 			}
// 			respCh <- resp
// 		}(i, endpoint)
// 	}

// 	var doneCount int
// 	for {
// 		select {
// 		case resp := <-respCh:
// 			cancel()
// 			return resp, nil
// 		case err := <-errCh:
// 			doneCount++
// 			if doneCount >= len(endpoints) {
// 				cancel()
// 				return nil, err
// 			}
// 		}
// 	}
// }

// func ListMembers(endpoints []string, tlsConfig *tls.Config) (*clientv3.MemberListResponse, error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
// 	client, err := newClient(ctx, endpoints, tlsConfig)
// 	if err != nil {
// 		return nil, fmt.Fatalf("list members failed: creating etcd client failed: %v", err)
// 	}
// 	defer client.Close()

// 	resp, err := client.MemberList(ctx)
// 	cancel()
// 	client.Close()
// 	return resp, err
// }
