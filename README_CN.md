# Discov

Discov 是通用的 gRPC 服务注册发现/负载均衡组件。

## 已支持的场景

- 以 etcd 作为服务注册中心的场景
- Kubernetes 集群中使用 Headless Service 作为 DNS name 发现服务的场景
- DNS 作为服务注册中心的场景

## 架构

![architecture](./arch.png)

## 使用示例

- [etcd 作注册中心的场景](./examples/etcd)
- [Kubernetes Headless Service 场景](./examples/k8s)

## 文档

[pkg.go.dev](https://pkg.go.dev/github.com/xvrzhao/discov)

## 开源协议

[MIT License](./LICENSE)