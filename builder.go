package discov

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc/resolver"
	"net"
	"sync"
	"time"
)

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

	if authority == "k8s.headless.svc" {
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
