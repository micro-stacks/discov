# discov

discov 是 k8s 与 非 k8s 场景下的通用 gRPC 服务注册发现/负载均衡组件。

## 使用场景

- **etcd** 利用 etcd 作为注册中心
- **k8s service** 通过声明 k8s service 的方式注册服务

在 k8s 中自行使用 etcd 做注册中心，也属于 etcd 场景。

## 客户端

本包提供的 resolver 默认采用 round robin 负载均衡策略，用户可自行实现 balancer 并通过 DialOptions 进行调整：

```
- grpc.WithDisableServiceConfig() // 不使用本包 resovler 的 service config
- grpc.WithDefaultServiceConfig() // 用户自己指定 service config
```

若 go get 提示含有 undefined type，是因为 grpc 在 v1.26.0 -> v1.27.0 的时候含有 API 不兼容更新，请修改 go.mod 将 grpc 版本降级到 v1.26.0

优化：resolver watch 到的服务地址若不可用，grpc 内部 ccResolverWrapper 会产生 panic，现解决方案是，服务端 shutdown 时需先将自己的地址从注册中心 unregister

编写 docker-compose YML 进行 example 负载均衡的演示。

## 服务端

本包提供的服务注册相关的接口，只提供 etcd 场景下使用。k8s 场景下使用 YML 声明 service 资源，相当于服务注册。
  