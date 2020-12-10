# Discovery Client

## 简介

DiscoveryClient 是发现客户端, 主要用来发现 Kubernetes API Server 所支持的资源组、资源版本、资源信息. 

kubectl 的 api-versions 和 api-resources 就是通过 DiscoveryClient 实现的. 
另外, DiscoveryClient 同样在RestClient上进行了封装. 

DiscoveryClient 可以将资源信息缓存在本地, 默认地址为 `~/.kube/cache` 和 `~/.kube/http-cache`. 
缓存可以减轻 Kubernetes API Server 的压力, 默认10分钟同步一次, 因为资源组、资源版本和资源信息变动较少. 

## 源码分析

TODO