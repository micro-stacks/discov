package discov

import (
	"context"
	"errors"
	"fmt"
	"go.etcd.io/etcd/clientv3"
	"google.golang.org/grpc/resolver"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	authorityEtcd = "etcd"
	authorityDNS  = "dns"
)

var (
	supportedAuthorities = []string{
		authorityEtcd,
		authorityDNS,
		// ...
	}

	errUnsupportedAuthorityInTarget = errors.New("unsupported authority in target")
	errEmptyAuthorityInTarget       = errors.New("empty authority in target")
	errInvalidEndpointInDNSTarget   = errors.New("invalid endpoint, must be in form of `DNSName:port`")
)

func isSupportedAuthority(a string) bool {
	for i := range supportedAuthorities {
		if a == supportedAuthorities[i] {
			return true
		}
	}

	return false
}

func parseTarget(t resolver.Target) (scheme, authority, endpoint string, err error) {
	scheme, authority, endpoint = t.Scheme, t.Authority, t.Endpoint

	if !isSupportedAuthority(authority) {
		err = errUnsupportedAuthorityInTarget
		return
	}

	if endpoint == "" {
		err = errEmptyAuthorityInTarget
		return
	}

	return
}

func parseEndpointInDNSTarget(endpoint string) (DNSName string, port int, err error) {
	s := strings.Split(endpoint, ":")

	// no port specified, default 80
	if len(s) == 1 {
		DNSName, port = s[0], 80
		return
	}

	if len(s) != 2 {
		err = errInvalidEndpointInDNSTarget
		return
	}

	port, err = strconv.Atoi(s[1])
	if err != nil {
		err = errInvalidEndpointInDNSTarget
		return
	}

	DNSName = s[0]
	return
}

func getEtcdKeyPrefix(srv string) (keyPrefix string) {
	keyPrefix = fmt.Sprintf("/srv/%s", srv)
	return
}

func formatIP(addr string) (addrIP string, ok bool) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "", false
	}

	if ip.To4() != nil {
		return addr, true
	}

	return "[" + addr + "]", true
}

func isEtcdClientAvailable(cli *clientv3.Client) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	if _, err := cli.Get(ctx, "ping"); err != nil {
		return false
	}

	return true
}

func makeErrorChannel() chan error {
	return make(chan error, 1<<6)
}
