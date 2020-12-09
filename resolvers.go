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

/********************************** k8s resolver **********************************/

// TODO: complete k8s resolver

/************************************* builder ************************************/

type builder struct {
	options *builderOptions
}

type builderOptions struct {
	etcdClient *clientv3.Client
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

func (b *builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	_, authority, endpoint, err := parseTarget(target)
	if err != nil {
		return nil, fmt.Errorf("parseTarget: %v", err)
	}

	var r resolver.Resolver

	if authority == "etcd" {
		if b.options.etcdClient == nil {
			return nil, fmt.Errorf(
				"the authority in target is %q but missing WithEtcdClient Option when NewBuilder", "etcd")
		}
		ctx, cancel := context.WithCancel(context.Background())
		r := &etcdResolver{
			cli:           b.options.etcdClient,
			srv:           endpoint,
			cc:            cc,
			ctx:           ctx,
			cancel:        cancel,
			disableSrvCfg: opts.DisableServiceConfig,
		}

		r.wg.Add(1)
		go r.watcher()
	} else if authority == "k8s" {
		// TODO: complete k8s logic
		return nil, nil
	} else {
		return nil, fmt.Errorf("invalid authority %q in target", authority)
	}

	return r, err
}

func (b *builder) Scheme() string {
	return "discov"
}

/********************************** builder options **********************************/

func WithEtcdClient(c *clientv3.Client) BuilderOption {
	return BuilderOption{
		applyTo: func(options *builderOptions) {
			options.etcdClient = c
		},
	}
}
