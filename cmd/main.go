package main

import (
	"github.com/randomcoww/etcd-wrapper/pkg/status"
	"time"
)

func main() {
	status, err := status.New()
	if err != nil {
		panic(err)
	}

	sync := make(chan struct{})
	syncDone := make(chan struct{})

	go func() {
		for {
			select {
			case <-sync:
				err = status.SyncStatus()
				if err != nil {
					panic(err)
				}
				syncDone <- struct{}{}
			}
		}
	}()

	go func() {
		for {
			sync <- struct{}{}
			<-syncDone

			if status.ClusterID == nil {
				// no cluster ID found
				// run restore
				err = status.WritePodManifest(true)
				if err != nil {

				}
				time.Sleep(1 * time.Minute)
				break
			}

			if !status.Healthy {
				err = status.WritePodManifest(true)
				if err != nil {

				}
				time.Sleep(1 * time.Minute)
				break
			}

			if status.ClusterID != status.MemberSelf.ClusterID {
				// do add remove
				err = status.ReplaceMember(status.MemberSelf)
				if err != nil {
					err = status.WritePodManifest(true)
					if err != nil {

					}
					time.Sleep(1 * time.Minute)
					break
				}
				err = status.WritePodManifest(false)
				if err != nil {

				}
				time.Sleep(1 * time.Minute)
				break
			}

			time.Sleep(6 * time.Second)
		}
	}()

	go func() {
		for {
			sync <- struct{}{}
			<-syncDone

			err = status.BackupSnapshot()
			if err != nil {

			}
			time.Sleep(12 * time.Minute)
		}
	}()

	<-make(chan struct{}, 1)
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
// 		return nil, fmt.Errorf("list members failed: creating etcd client failed: %v", err)
// 	}
// 	defer client.Close()

// 	resp, err := client.MemberList(ctx)
// 	cancel()
// 	client.Close()
// 	return resp, err
// }
