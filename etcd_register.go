package discov

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.etcd.io/etcd/clientv3"
)

// EtcdSrvRecord is a service record to be registered to or
// unregistered from etcd.
type EtcdSrvRecord interface {
	// Register registers the record to etcd.
	// This method should be called when the service starts.
	Register()
	// Unregister unregisters the record from etcd.
	// This method should be called when the service is closed.
	Unregister(context.Context) error
	// CatchRuntimeErrors catches runtime errors.
	// Since the EtcdSrvRecord spawns goroutines to keep heartbeat
	// to etcd after calling Register, some errors may occur at runtime.
	CatchRuntimeErrors() <-chan error
	// RegisterKvBuilder injects an EtcdKvBuilder to custom the
	// format of key and value to store.
	RegisterKvBuilder(EtcdKvBuilder)
}

// NewEtcdSrvRecord creates a service record.
func NewEtcdSrvRecord(cli *clientv3.Client, srvName, srvAddr string) (EtcdSrvRecord, error) {
	if cli == nil {
		return nil, errors.New("nil etcd client")
	}
	if srvName == "" {
		return nil, errors.New("empty srv name")
	}
	if srvAddr == "" {
		return nil, errors.New("empty srv addr")
	}

	if !isEtcdClientAvailable(cli) {
		return nil, errors.New("unavailable etcd client")
	}

	r := &etcdSrvRecord{
		cli:       cli,
		ctx:       nil,
		cancel:    nil,
		wg:        sync.WaitGroup{},
		errs:      nil,
		errsLock:  sync.Mutex{},
		srvName:   srvName,
		srvAddr:   srvAddr,
		kvBuilder: nil,
		srvTTL:    time.Second * 3,
	}

	return r, nil
}

type etcdSrvRecord struct {
	cli              *clientv3.Client
	srvTTL           time.Duration
	kvBuilder        EtcdKvBuilder
	srvName, srvAddr string

	// used as revoking checker, aliveKeeper and consumer
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	errs     chan error // note: never close after initialization
	errsLock sync.Mutex
}

func (r *etcdSrvRecord) Register() {
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.initErrorChannel()

	if r.kvBuilder == nil {
		r.RegisterKvBuilder(new(DefaultEtcdKvBuilder))
	}

	r.wg.Add(1)
	go r.checker()
}

func (r *etcdSrvRecord) Unregister(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel() // revokes checker, aliveKeeper and consumer
		r.wg.Wait()
	}

	if r.kvBuilder == nil {
		r.RegisterKvBuilder(new(DefaultEtcdKvBuilder))
	}

	key := r.kvBuilder.BuildKey(r.srvName, r.srvAddr)
	_, err := r.cli.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("etcd client delete key(%s): %v", key, err)
	}

	return nil
}

func (r *etcdSrvRecord) CatchRuntimeErrors() <-chan error {
	r.initErrorChannel()

	return r.errs
}

// RegisterKvBuilder should be called before Register and Unregister.
func (r *etcdSrvRecord) RegisterKvBuilder(b EtcdKvBuilder) {
	r.kvBuilder = b
}

func (r *etcdSrvRecord) checker() {
	defer r.wg.Done()

	key := r.kvBuilder.BuildKey(r.srvName, r.srvAddr)
	ticker := time.NewTicker(r.srvTTL)

	for {
		func() {
			ctx, cancel := context.WithTimeout(r.ctx, r.srvTTL)
			defer cancel()

			resp, err := r.cli.Get(ctx, key)
			if err != nil {
				r.pushErr(fmt.Errorf("CHECKER_ERROR: failed to get key: %v", err))
				return
			}

			if resp.Count <= 0 {
				lease, err := r.cli.Grant(ctx, r.srvTTL.Milliseconds()/1000)
				if err != nil {
					r.pushErr(fmt.Errorf("CHECKER_ERROR: failed to grant lease: %v", err))
					return
				}

				val := r.kvBuilder.BuildValue(r.srvName, r.srvAddr)
				_, err = r.cli.Put(ctx, key, val, clientv3.WithLease(lease.ID))
				if err != nil {
					r.pushErr(fmt.Errorf("CHECKER_ERROR: failed to put key: %v", err))
					return
				}

				ch, err := r.cli.KeepAlive(r.ctx, lease.ID) // aliveKeeper
				if err != nil {
					r.pushErr(fmt.Errorf("CHECKER_ERROR: failed to keep alive: %v", err))
					return
				}

				r.wg.Add(1)
				go r.consumer(ch)
			}
		}()

		select {
		case <-r.ctx.Done():
			// revoke this goroutine
			ticker.Stop()
			return
		case <-ticker.C:
		}
	}
}

func (r *etcdSrvRecord) consumer(ch <-chan *clientv3.LeaseKeepAliveResponse) {
	defer r.wg.Done()

	for {
		select {
		case <-r.ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				r.pushErr(errors.New("KEEP_ALIVE_RESP_CONSUMER_WARNING: underlying keep alive stream is interrupted"))
				return
			}
		}
	}
}

func (r *etcdSrvRecord) pushErr(err error) {
	r.initErrorChannel()

	select {
	case r.errs <- err:
	default: // channel is full, discard err
	}
}

func (r *etcdSrvRecord) initErrorChannel() {
	r.errsLock.Lock()
	defer r.errsLock.Unlock()

	if r.errs == nil {
		r.errs = makeErrorChannel()
	}
}
