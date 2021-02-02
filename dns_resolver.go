package discov

import (
	"context"
	"fmt"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/resolver"
	"net"
	"sync"
	"time"
)

// dnsResolver is an implementation of gRPC Resolver interface.
type dnsResolver struct {
	dnsName         string
	port            int
	resolver        *net.Resolver
	pollingInterval time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	rn              chan struct{}
	cc              resolver.ClientConn
	disableSrvCfg   bool
}

func (r *dnsResolver) ResolveNow(resolver.ResolveNowOptions) {
	select {
	case r.rn <- struct{}{}:
	default:
	}
}

func (r *dnsResolver) Close() {
	r.cancel()
	r.wg.Wait()
}

func (r *dnsResolver) watcher() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.pollingInterval)
	for {
		select {
		case <-r.ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
		case <-r.rn:
		}

		r.resolve()
	}
}

func (r *dnsResolver) resolve() {
	stat := resolver.State{}

	ctx, cancel := context.WithTimeout(r.ctx, time.Second*3)
	defer cancel()

	addrs, err := r.resolver.LookupHost(ctx, r.dnsName)
	if err != nil {
		r.cc.ReportError(fmt.Errorf("dns resolver lookup host failed: %v", err))
		return
	}

	var newAddrs []resolver.Address
	for _, addr := range addrs {
		addr, ok := formatIP(addr)
		if !ok {
			continue
		}
		addr = fmt.Sprintf("%s:%d", addr, r.port)
		newAddrs = append(newAddrs, resolver.Address{Addr: addr})
	}

	stat.Addresses = newAddrs
	if !r.disableSrvCfg {
		stat.ServiceConfig = r.cc.ParseServiceConfig(fmt.Sprintf(`{"LoadBalancingPolicy": %q}`, roundrobin.Name))
	}

	r.cc.UpdateState(stat)
}
