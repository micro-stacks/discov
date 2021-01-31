package main

import (
	"context"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/xvrzhao/discov"
	pb "github.com/xvrzhao/discov/examples/proto"
	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
	"os"
	"time"
)

const (
	etcdAddr = "etcd:2379"
)

var greetingSrv pb.GreetingClient

func init() {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stdout, os.Stderr, os.Stderr))

	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{etcdAddr}, DialTimeout: time.Second})
	if err != nil {
		panic(err)
	}

	resolver.Register(discov.NewBuilder(discov.WithEtcdClient(cli)))

	greetingClientConn, err := grpc.Dial("discov://etcd/greeting", grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		panic(err)
	}

	greetingSrv = pb.NewGreetingClient(greetingClientConn)
}

func main() {
	grpclog.Infoln("rpc client start")

	for range time.Tick(time.Second) {
		resp, err := greetingSrv.Greet(context.TODO(), new(empty.Empty))
		if err != nil {
			grpclog.Errorln(err)
		}

		grpclog.Infof("receive response: %s\n", resp.Value)
	}
}
