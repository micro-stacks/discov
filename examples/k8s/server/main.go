package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/wrappers"
	pb "github.com/xvrzhao/discov/examples/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"net"
	"os"
)

const (
	listenPort = 8080
)

var intranetIP string

type GreetingServer struct{}

func (g GreetingServer) Greet(_ context.Context, _ *empty.Empty) (*wrappers.StringValue, error) {
	return &wrappers.StringValue{Value: intranetIP}, nil
}

func GetIntranetIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("net.InterfaceAddrs: %v", err)
	}

	for _, addr := range addrs {
		if IPNet, ok := addr.(*net.IPNet); ok && !IPNet.IP.IsLoopback() {
			if IPv4 := IPNet.IP.To4(); IPv4 != nil {
				return IPv4.String(), nil
			}
		}
	}

	return "", errors.New("unable to get the intranet IP")
}

func init() {
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stdout, os.Stderr, os.Stderr))

	var err error
	intranetIP, err = GetIntranetIP()
	if err != nil {
		panic(err)
	}
}

func main() {
	s := grpc.NewServer()
	pb.RegisterGreetingServer(s, new(GreetingServer))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		panic(err)
	}

	grpclog.Infoln("rpc server start")
	if err = s.Serve(lis); err != nil {
		panic(err)
	}
}
