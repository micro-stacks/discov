package discov

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

var cli *clientv3.Client

func TestNewEtcdSrvRecord(t *testing.T) {
	r, err := NewEtcdSrvRecord(cli, "test", "1.2.3.4:8080")
	if err != nil {
		t.Errorf("failed to new record: %v", err)
	}

	r.Register()

	go func() {
		for err := range CatchRuntimeErrors() {
			fmt.Println(err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT)
	go func() {
		<-ch
		if err := r.Unregister(context.TODO()); err != nil {
			t.Errorf("failed to unregister: %v", err)
		}
	}()

	select {}
}

func init() {
	var err error
	cli, err = clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: time.Second,
	})
	if err != nil {
		panic(err)
	}
}
