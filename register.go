package discov

import (
	"context"
	"errors"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"time"
)

// todo integrate CatchErrors into implementation
// EtcdSrvRecord is a service record to be registered to or unregistered from etcd.
type EtcdSrvRecord interface {
	// Register registers the record to etcd.
	// This method should be called when the service starts.
	Register()
	// Unregister unregisters the record from etcd.
	// This method should be called when the service is closed.
	Unregister(context.Context) error
}

// NewEtcdSrvRecord construct a service record.
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
		cli:     cli,
		ctx:     nil,
		cancel:  nil,
		srvName: srvName,
		srvAddr: srvAddr,
		srvTTL:  time.Second * 3,
	}

	return r, nil
}

type etcdSrvRecord struct {
	cli *clientv3.Client

	// used as revoking checker, aliveKeeper and consumer
	ctx    context.Context
	cancel context.CancelFunc

	srvName string
	srvAddr string
	srvTTL  time.Duration
}

func (r *etcdSrvRecord) Register() {
	r.ctx, r.cancel = context.WithCancel(context.Background())
	go r.checker()
}

func (r *etcdSrvRecord) Unregister(ctx context.Context) error {
	if r.cancel != nil {
		r.cancel() // revokes checker, aliveKeeper and consumer
	}

	key := fmt.Sprintf("%s/%v", getEtcdKeyPrefix(r.srvName), r.srvAddr)
	_, err := r.cli.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("etcd client delete key(%s): %v", key, err)
	}

	return nil
}

func (r *etcdSrvRecord) checker() {

	key := fmt.Sprintf("%s/%v", getEtcdKeyPrefix(r.srvName), r.srvAddr)
	ticker := time.NewTicker(r.srvTTL)

	for {
		select {
		case <-r.ctx.Done():
			// revoke this goroutine
			ticker.Stop()
			return
		default:
			ctx, cancel := context.WithTimeout(context.Background(), r.srvTTL)
			resp, err := r.cli.Get(ctx, key)
			if err != nil {
				cancel()
				pushError(fmt.Errorf("CHECKER_ERROR: failed to get key: %v", err))
				break
			}

			if resp.Count <= 0 {
				lease, err := r.cli.Grant(ctx, r.srvTTL.Milliseconds()/1000)
				if err != nil {
					cancel()
					pushError(fmt.Errorf("CHECKER_ERROR: failed to grant lease: %v", err))
					break
				}

				_, err = r.cli.Put(ctx, key, r.srvAddr, clientv3.WithLease(lease.ID))
				if err != nil {
					cancel()
					pushError(fmt.Errorf("CHECKER_ERROR: failed to put key: %v", err))
					break
				}

				ch, err := r.cli.KeepAlive(r.ctx, lease.ID) // aliveKeeper
				if err != nil {
					cancel()
					pushError(fmt.Errorf("CHECKER_ERROR: failed to keep alive: %v", err))
					break
				}

				go r.consumer(ch)
			}

			cancel()
		}

		<-ticker.C
	}
}

func (r *etcdSrvRecord) consumer(ch <-chan *clientv3.LeaseKeepAliveResponse) {
	for {
		select {
		case <-r.ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				pushError(errors.New("KEEP_ALIVE_RESP_CONSUMER_WARNING: underlying keep alive stream is interrupted"))
				return
			}
		}
	}
}
