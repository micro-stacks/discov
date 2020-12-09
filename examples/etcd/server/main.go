package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/wrappers"
	pb "github.com/xvrzhao/discov/examples/proto"
	"google.golang.org/grpc"
	"net"
)

type GreetingServer struct{}

func (g GreetingServer) Greet(ctx context.Context, value *wrappers.UInt32Value) (*empty.Empty, error) {
	fmt.Println(value.Value)
	return new(empty.Empty), nil
}

var port int

func init() {
	flag.IntVar(&port, "p", 9000, "")
	flag.Parse()
}

func main() {
	gServer := grpc.NewServer()
	pb.RegisterGreetingServer(gServer, new(GreetingServer))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}

	if err = gServer.Serve(lis); err != nil {
		panic(err)
	}
}
