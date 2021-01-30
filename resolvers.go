package discov

import (
	"context"
	"errors"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc/balancer/roundrobin"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
	"net"
	"sync"
	"time"
)

/********************************** etcd resolver **********************************/

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

/********************************** k8s.headless.svc resolver **********************************/

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

/************************************* builder ************************************/

type builder struct {
	options *builderOptions
}

type builderOptions struct {
	// etcd
	etcdClient *clientv3.Client
	// k8s headless svc
	headlessLookupFrequency time.Duration
}

type BuilderOption struct {
	applyTo func(*builderOptions)
}

func NewBuilder(opts ...BuilderOption) *builder {
	options := new(builderOptions)
	for _, option := range opts {
		option.applyTo(options)
	}
	return &builder{options: options}
}

func (b *builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (r resolver.Resolver, err error) {
	_, authority, endpoint, err := parseTarget(target)
	if err != nil {
		err = fmt.Errorf("parseTarget: %v", err)
		return
	}

	if authority == "etcd" {
		if b.options.etcdClient == nil {
			err = fmt.Errorf("the authority in target is %q but missing WithEtcdClient option when NewBuilder", authority)
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		er := &etcdResolver{
			cli:           b.options.etcdClient,
			srv:           endpoint,
			cc:            cc,
			ctx:           ctx,
			cancel:        cancel,
			disableSrvCfg: opts.DisableServiceConfig,
		}

		er.wg.Add(1)
		go er.watcher()

		r = er
		return
	}

	if authority == "headless" {
		var host, port string
		host, port, err = parseEndpoint(endpoint)
		if err != nil {
			err = fmt.Errorf("parseEndpoint: %v", err)
			return
		}
		if b.options.headlessLookupFrequency == 0 {
			err = fmt.Errorf("the authority in target is %q but missing WithHeadlessLookupFrequency option when NewBuilder", authority)
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		kr := &k8sHeadlessSvcResolver{
			host:          host,
			port:          port,
			resolver:      net.DefaultResolver,
			ctx:           ctx,
			cancel:        cancel,
			cc:            cc,
			rn:            make(chan struct{}),
			freq:          b.options.headlessLookupFrequency,
			wg:            sync.WaitGroup{},
			disableSrvCfg: opts.DisableServiceConfig,
		}

		kr.wg.Add(1)
		go kr.watcher()
		kr.ResolveNow(resolver.ResolveNowOptions{})

		r = kr
		return
	}

	err = fmt.Errorf("invalid authority %q in target", authority)
	return
}

func (b *builder) Scheme() string {
	return "discov"
}

/********************************** builder options **********************************/

func WithEtcdClient(cli *clientv3.Client) BuilderOption {
	return BuilderOption{
		applyTo: func(options *builderOptions) {
			options.etcdClient = cli
		},
	}
}

func WithHeadlessLookupFrequency(d time.Duration) BuilderOption {
	return BuilderOption{
		applyTo: func(options *builderOptions) {
			options.headlessLookupFrequency = d
		},
	}
}
