package main

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/wrappers"
	pb "github.com/xvrzhao/discov/examples/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"net"
	"os"
)

type GreetingServer struct{}

func (g GreetingServer) Greet(ctx context.Context, e *empty.Empty) (*wrappers.StringValue, error) {
	return &wrappers.StringValue{Value: fmt.Sprintf("Hello from %s", os.Getenv("POD_NAME"))}, nil
}

func main() {
	gServer := grpc.NewServer()
	pb.RegisterGreetingServer(gServer, new(GreetingServer))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 8080))
	if err != nil {
		panic(err)
	}

	grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stdout, os.Stderr, os.Stderr))
	grpclog.Infoln("grpc server start")

	if err = gServer.Serve(lis); err != nil {
		panic(err)
	}
}
