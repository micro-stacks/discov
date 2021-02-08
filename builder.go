package discov

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc/resolver"
)

// Builder implements the gRPC resolver.Builder interface.
// It Builds a discov Resolver that implements the gRPC
// resolver.Resolver interface.
type Builder struct {
	options *builderOptions
}

type builderOptions struct {
	customScheme string
	// etcd
	etcdClient *clientv3.Client
	kvResolver EtcdKvResolver
	// dns
	dnsPollingInterval time.Duration
}

// BuilderOption is the option that passed to NewBuilder function.
type BuilderOption struct {
	applyTo func(*builderOptions)
}

// WithEtcdClient injects an etcd client. The option is required
// when the authority of discov is etcd.
func WithEtcdClient(cli *clientv3.Client) BuilderOption {
	return BuilderOption{
		applyTo: func(options *builderOptions) {
			options.etcdClient = cli
		},
	}
}

// WithEtcdKvResolver injects a custom EtcdKvResolver implementation
// to determine how the internal etcdResolver watches keys and retrieves addrs.
func WithEtcdKvResolver(r EtcdKvResolver) BuilderOption {
	return BuilderOption{
		applyTo: func(options *builderOptions) {
			options.kvResolver = r
		},
	}
}

// WithDNSPollingInterval customizes the polling frequency of
// the internal dnsResolver.
func WithDNSPollingInterval(d time.Duration) BuilderOption {
	return BuilderOption{
		applyTo: func(options *builderOptions) {
			options.dnsPollingInterval = d
		},
	}
}

// WithCustomScheme customizes scheme name of the internal protocol.
func WithCustomScheme(scheme string) BuilderOption {
	return BuilderOption{
		applyTo: func(options *builderOptions) {
			options.customScheme = scheme
		},
	}
}

// NewBuilder returns a Builder that contain the passed options.
func NewBuilder(opts ...BuilderOption) *Builder {
	options := new(builderOptions)
	for _, option := range opts {
		option.applyTo(options)
	}
	return &Builder{options: options}
}

// Scheme returns the scheme correspond to the Builder.
func (b *Builder) Scheme() string {
	if b.options.customScheme != "" {
		return b.options.customScheme
	}
	return defaultScheme
}

// Build creates a new resolver for the given target.
//
// If the authority of target that passed to grpc dial method
// is etcd, Build returns an internal etcdResolver, if the
// authority is dns, returns an internal dnsResolver.
//
// The options passed to NewBuilder will be used in this method.
func (b *Builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (r resolver.Resolver, err error) {
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

		if b.options.kvResolver == nil {
			b.options.kvResolver = new(DefaultEtcdKvResolver)
		}

		ctx, cancel := context.WithCancel(context.Background())

		er := &etcdResolver{
			cli:           b.options.etcdClient,
			srv:           endpoint,
			kvResolver:    b.options.kvResolver,
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
			b.options.dnsPollingInterval = time.Second * 30
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
