package main

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/xvrzhao/discov"
	"github.com/xvrzhao/discov/examples/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"log"
	"time"
)

// TODO: improve k8s examples

var cc *grpc.ClientConn

func init() {
	resolver.Register(discov.NewBuilder(discov.WithHeadlessLookupFrequency(time.Second)))

	var err error
	ctx, _ := context.WithTimeout(context.Background(), time.Second*3)
	cc, err = grpc.DialContext(ctx, "discov://k8s.headless.svc/greeting-svc:8080", grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
}

func main() {
	fmt.Println("client run")
	cli := proto.NewGreetingClient(cc)
	for range time.Tick(time.Second) {
		res, err := cli.Greet(context.Background(), new(empty.Empty))
		if err != nil {
			log.Print(err)
			continue
		}
		fmt.Println(res.Value)
	}
}
