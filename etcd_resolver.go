package discov

import (
	"context"
	"errors"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/resolver"
	"sync"
)

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
	r.cancel()
	r.wg.Wait()
}

func (r *etcdResolver) watcher() {
	defer r.wg.Done()

	prefix := getEtcdKeyPrefix(r.srv)
	r.update(prefix)

	ch := r.cli.Watch(r.ctx, prefix, clientv3.WithPrefix())
	for {
		select {
		case rsp, ok := <-ch:
			if !ok {
				r.cc.ReportError(errors.New("resolver watcher: watch channel has been closed"))
				return
			}
			if rsp.Canceled {
				r.cc.ReportError(errors.New("resolver watcher: watch channel receive a canceled response"))
				return
			}
			if rsp.Err() != nil {
				r.cc.ReportError(fmt.Errorf("resolver watcher: watch channel receive error: %v", rsp.Err()))
				return
			}
			r.update(prefix)
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *etcdResolver) update(keyPrefix string) {
	rsp, err := r.cli.Get(r.ctx, keyPrefix, clientv3.WithPrefix())
	if err != nil {
		r.cc.ReportError(fmt.Errorf("resolver update: %v", err))
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
