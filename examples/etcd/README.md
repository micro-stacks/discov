## Examples of etcd scenario

Examples of usage when etcd is used as the registry.

These examples demonstrate discovery on the client side and registration on the server side.
You can start the two programs and observe their behavior through logs.

### Commands

Execute the following commands under `examples/etcd/`.

```
# startup the client and the server
$ docker-compose -p discov_demo up -d

# view client logs
$ docker-compose -p discov_demo logs -f rpc_client

# view server logs
$ docker-compose -p discov_demo logs -f rpc_server

# scale the number of server instances to test the dynamic adjustment of the example system
$ docker-compose -p discov_demo up -d --scale rpc_server=${NUM}

# stop and remove the examples 
$ docker-compose -p discov_demo down --rmi local
```
