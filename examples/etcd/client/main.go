package main

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/micro-stacks/discov"
	pb "github.com/micro-stacks/discov/examples/proto"
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

type etcdKvResolver struct{}

func (r *etcdKvResolver) GetKeyPrefixForSrv(srvName string) (prefix string) {
	return fmt.Sprintf("/srvs/%s", srvName)
}

func (r *etcdKvResolver) ResolveSrvAddr(value []byte) (srvAddr string) {
	return string(value)
}

func init() {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stdout, os.Stderr, os.Stderr))

	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{etcdAddr}, DialTimeout: time.Second})
	if err != nil {
		panic(err)
	}

	resolver.Register(discov.NewBuilder(
		discov.WithEtcdClient(cli),
		discov.WithEtcdKvResolver(new(etcdKvResolver)),
	))

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
			grpclog.Errorf("call greet failed: %v\n", err)
			continue
		}

		grpclog.Infof("receive response: %s\n", resp.Value)
	}
}
