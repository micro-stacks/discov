package main

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/xvrzhao/discov"
	pb "github.com/xvrzhao/discov/examples/proto"
	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"log"
	"strings"
	"time"
)

const (
	etcdEndpoints = "127.0.0.1:2379"
)

// greetingClientConn is a process-level variable, it used by all greetingClient for multiplexing.
var greetingClientConn *grpc.ClientConn

// other ClientConns ...

func init() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   strings.Split(etcdEndpoints, ","),
		DialTimeout: time.Second * 3,
	})
	if err != nil {
		panic(err)
	}
	// note: should not close cli, it always being used by resolver

	resolver.Register(discov.NewBuilder(discov.WithEtcdClient(cli)))

	greetingClientConn, err = grpc.Dial("discov://etcd/greeting", grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	// dial other services ...
}

func main() {
	fmt.Println("start")

	cli := pb.NewGreetingClient(greetingClientConn)

	for now := range time.Tick(time.Second) {
		_, err := cli.Greet(context.Background(), &wrappers.UInt32Value{Value: uint32(now.Second())})
		if err != nil {
			log.Print(err)
		}
	}
}
