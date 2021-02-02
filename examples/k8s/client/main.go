package main

import (
	"context"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/xvrzhao/discov"
	pb "github.com/xvrzhao/discov/examples/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
	"os"
	"time"
)

var greetingSrv pb.GreetingClient

func init() {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stdout, os.Stderr, os.Stderr))

	resolver.Register(discov.NewBuilder(discov.WithDNSPollingInterval(time.Second)))

	greetingClientConn, err := grpc.Dial("discov://dns/greeting-svc:8080", grpc.WithBlock(), grpc.WithInsecure())
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
