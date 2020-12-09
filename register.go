package discov

import (
	"context"
	"errors"
	"fmt"
	"go.etcd.io/etcd/clientv3"
)

func Register(ctx context.Context, cli *clientv3.Client, srv, addr string, ttl int) error {
	if cli == nil {
		return errors.New("nil etcd client")
	}

	grantRsp, err := cli.Grant(ctx, int64(ttl))
	if err != nil {
		return fmt.Errorf("etcd client grant: %v", err)
	}

	k := fmt.Sprintf("%s/%s", getEtcdKeyPrefix(srv), addr)
	putRsp, err := cli.Put(ctx, k, addr, clientv3.WithLease(grantRsp.ID))
	if err != nil {
		return fmt.Errorf("etcd client put: %v", err)
	}

	cli.KeepAliveOnce()
}
