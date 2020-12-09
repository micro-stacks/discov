package discov

import (
	"errors"
	"fmt"
	"google.golang.org/grpc/resolver"
)

func parseTarget(t resolver.Target) (scheme, authority, endpoint string, err error) {
	scheme, authority, endpoint = t.Scheme, t.Authority, t.Endpoint
	if scheme != "discov" {
		err = fmt.Errorf("the scheme %q matched the discov builder incorrectly", scheme)
		return
	}
	if authority != "etcd" && authority != "k8s" {
		err = fmt.Errorf("invalid authority, must be %q or %q", "etcd", "k8s")
		return
	}
	if endpoint == "" {
		err = errors.New("endpoint is empty")
		return
	}
	return
}

func getEtcdKeyPrefix(srv string) (keyPrefix string) {
	keyPrefix = fmt.Sprintf("/srv/%s", srv)
	return
}
