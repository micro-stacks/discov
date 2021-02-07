package discov

import "fmt"

// EtcdKvBuilder builds the key and the value to store to etcd.
type EtcdKvBuilder interface {
	// BuildKey builds the etcd key according to the name and
	// the address of the service.
	BuildKey(srvName, srvAddr string) (key string)
	// BuildValue builds the etcd value according to the name
	// and the address of the service.
	BuildValue(srvName, srvAddr string) (value string)
}

// DefaultEtcdKvBuilder will be used if the user does not RegisterKvBuilder.
type DefaultEtcdKvBuilder struct{}

// BuildKey returns the key of "/srv/${srvName}/${srvAddr}".
func (*DefaultEtcdKvBuilder) BuildKey(srvName, srvAddr string) string {
	return fmt.Sprintf("/srv/%s/%s", srvName, srvAddr)
}

// BuildValue returns the value as the same as srvAddr.
func (*DefaultEtcdKvBuilder) BuildValue(_, srvAddr string) string {
	return srvAddr
}

// EtcdKvResolver determines how the internal etcdResolver
// watches keys and retrieves addrs.
type EtcdKvResolver interface {
	// GetKeyPrefixForSrv generates the key prefix for etcdResolver to watch.
	// srvName is the endpoint in the target that passed to the grpc Dial method.
	GetKeyPrefixForSrv(srvName string) (prefix string)
	// ResolveSrvAddr resolves the service address from the value corresponding
	// to the etcd key that be watched.
	ResolveSrvAddr(value []byte) (srvAddr string)
}

// DefaultEtcdKvResolver will be used if the user doesn't set the EtcdKvResolver.
// DefaultEtcdKvResolver watches keys with prefix of "/srv/${srvName}", and
// treats the value of the key directly as an address.
type DefaultEtcdKvResolver struct{}

// GetKeyPrefixForSrv returns the prefix of "/srv/${srvName}".
func (*DefaultEtcdKvResolver) GetKeyPrefixForSrv(srvName string) string {
	return fmt.Sprintf("/srv/%s", srvName)
}

// ResolveSrvAddr returns the string content of value.
func (r *DefaultEtcdKvResolver) ResolveSrvAddr(value []byte) string {
	return string(value)
}
