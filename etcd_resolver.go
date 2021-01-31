package discov

import (
	"context"
	"errors"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/resolver"
	"sync"
	"time"
)

// etcdResolver is an implementation of gRPC Resolver interface.
type etcdResolver struct {
	cli           *clientv3.Client
	srv           string
	cc            resolver.ClientConn
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	disableSrvCfg bool
}

func (r *etcdResolver) ResolveNow(resolver.ResolveNowOptions) {}

func (r *etcdResolver) Close() {
	r.cancel()  // revoke watcher
	r.wg.Wait() // wait watcher to be done
}

func (r *etcdResolver) watcher() {
	defer r.wg.Done()

	prefix := getEtcdKeyPrefix(r.srv)
	r.update(prefix)

	ch := r.cli.Watch(r.ctx, prefix, clientv3.WithPrefix())
	for {
		select {
		case <-r.ctx.Done():
			return
		case rsp, ok := <-ch:
			if !ok {
				r.cc.ReportError(errors.New("the watcher of etcdResolver: WatchResponse channel has been closed"))
				return
			}
			if rsp.Err() != nil {
				r.cc.ReportError(fmt.Errorf("resolver watcher: WatchResponse holds an error: %v", rsp.Err()))
				return
			}
			r.update(prefix)
		}
	}
}

func (r *etcdResolver) update(keyPrefix string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	rsp, err := r.cli.Get(ctx, keyPrefix, clientv3.WithPrefix())
	if err != nil {
		r.cc.ReportError(fmt.Errorf("resolver update: get keys: %v", err))
		return
	}

	stat := resolver.State{}

	var addrs []resolver.Address
	for _, kv := range rsp.Kvs {
		addrs = append(addrs, resolver.Address{Addr: string(kv.Value)})
	}
	stat.Addresses = addrs

	if !r.disableSrvCfg {
		stat.ServiceConfig = r.cc.ParseServiceConfig(fmt.Sprintf(`{"LoadBalancingPolicy": %q}`, roundrobin.Name))
	}

	r.cc.UpdateState(stat)
}
