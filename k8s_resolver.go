package discov

import (
	"context"
	"fmt"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
	"net"
	"sync"
	"time"
)

// TODO: code review and improvement

type k8sHeadlessSvcResolver struct {
	host          string
	port          string
	resolver      *net.Resolver
	ctx           context.Context
	cancel        context.CancelFunc
	cc            resolver.ClientConn
	rn            chan struct{}
	freq          time.Duration
	wg            sync.WaitGroup
	disableSrvCfg bool
}

func (r *k8sHeadlessSvcResolver) ResolveNow(resolver.ResolveNowOptions) {
	select {
	case r.rn <- struct{}{}:
	default:
	}
}

// Close closes the dnsResolver.
func (r *k8sHeadlessSvcResolver) Close() {
	r.cancel()
	r.wg.Wait()
}

func (r *k8sHeadlessSvcResolver) watcher() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.freq)
	for {
		select {
		case <-r.ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
		case <-r.rn:
		}

		state := r.lookup()
		r.cc.UpdateState(state)
	}
}

func (r *k8sHeadlessSvcResolver) lookup() resolver.State {
	stat := resolver.State{}

	addrs, err := r.resolver.LookupHost(r.ctx, r.host)
	if err != nil {
		grpclog.Errorf("resolver lookup host: %v", err)
		return stat
	}

	var newAddrs []resolver.Address
	for _, a := range addrs {
		a, ok := formatIP(a)
		if !ok {
			grpclog.Warningf("resolver format IP: invalid IP %s", a)
			continue
		}
		addr := a + ":" + r.port
		newAddrs = append(newAddrs, resolver.Address{Addr: addr})
	}

	stat.Addresses = newAddrs
	if !r.disableSrvCfg {
		stat.ServiceConfig = r.cc.ParseServiceConfig(fmt.Sprintf(`{"LoadBalancingPolicy": %q}`, roundrobin.Name))
	}

	return stat
}