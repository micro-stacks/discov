package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/xvrzhao/discov"
	pb "github.com/xvrzhao/discov/examples/proto"
	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	listenPort = 8080
	etcdAddr   = "etcd:2379"
	srvName    = "greeting"
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

func registerSrv() {
	grpclog.Infoln("register the service")

	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{etcdAddr}, DialTimeout: time.Second})
	if err != nil {
		panic(err)
	}

	record, err := discov.NewEtcdSrvRecord(cli, srvName, fmt.Sprintf("%s:%d", intranetIP, listenPort))
	if err != nil {
		panic(err)
	}
	record.Register()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL, syscall.SIGHUP)
	go func() {
		sig := <-ch
		grpclog.Infof("receive process signal: %v, unregister the service\n", sig)
		if err = record.Unregister(context.TODO()); err != nil {
			grpclog.Errorf("unregister failed: %v\n", err)
		}
	}()

	go func() {
		for err := range record.CatchRuntimeErrors() {
			grpclog.Errorf("catch runtime error of service register: %v\n", err)
		}
	}()
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

	go registerSrv()

	grpclog.Infoln("rpc server start")
	if err = s.Serve(lis); err != nil {
		panic(err)
	}
}
