package discov

import (
	"context"
	"errors"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc/resolver"
	"net"
	"sync"
	"time"
)

// builder implements the gRPC interface of resolver.Builder.
type builder struct {
	options *builderOptions
}

type builderOptions struct {
	// etcd
	etcdClient *clientv3.Client
	// dns
	dnsPollingInterval time.Duration
}

type BuilderOption struct {
	applyTo func(*builderOptions)
}

func WithEtcdClient(cli *clientv3.Client) BuilderOption {
	return BuilderOption{
		applyTo: func(options *builderOptions) {
			options.etcdClient = cli
		},
	}
}

func WithDNSPollingInterval(d time.Duration) BuilderOption {
	return BuilderOption{
		applyTo: func(options *builderOptions) {
			options.dnsPollingInterval = d
		},
	}
}

func NewBuilder(opts ...BuilderOption) *builder {
	options := new(builderOptions)
	for _, option := range opts {
		option.applyTo(options)
	}
	return &builder{options: options}
}

func (b *builder) Scheme() string {
	return "discov"
}

func (b *builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (r resolver.Resolver, err error) {
	_, authority, endpoint, err := parseTarget(target)
	if err != nil {
		err = fmt.Errorf("parse target: %v", err)
		return
	}

	if authority == authorityEtcd {
		if b.options.etcdClient == nil {
			err = fmt.Errorf("the authority in target is %q but missing WithEtcdClient option when NewBuilder",
				authority)
			return
		}

		if !isEtcdClientAvailable(b.options.etcdClient) {
			err = errors.New("the passed etcd client is unavailable")
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

	if authority == authorityDNS {
		var (
			dnsName string
			port    int
		)

		dnsName, port, err = parseEndpointInDNSTarget(endpoint)
		if err != nil {
			err = fmt.Errorf("parse endpoint failed: %v", err)
			return
		}

		if b.options.dnsPollingInterval == 0 {
			err = fmt.Errorf("the authority in target is %q but missing WithDNSPollingInterval option when NewBuilder",
				authority)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		dr := &dnsResolver{
			dnsName:         dnsName,
			port:            port,
			resolver:        net.DefaultResolver,
			pollingInterval: b.options.dnsPollingInterval,
			ctx:             ctx,
			cancel:          cancel,
			wg:              sync.WaitGroup{},
			cc:              cc,
			rn:              make(chan struct{}, 1),
			disableSrvCfg:   opts.DisableServiceConfig,
		}

		dr.wg.Add(1)
		go dr.watcher()
		dr.ResolveNow(resolver.ResolveNowOptions{})

		r = dr
		return
	}

	err = errUnsupportedAuthorityInTarget
	return
}
