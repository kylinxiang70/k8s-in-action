# Discovery Client

## 简介

DiscoveryClient是发现客户端，主要用来发现Kubernetes API Server所支持的资源组、资源版本、资源信息。

kubectl的api-versions和api-resources就是通过DiscoveryClient实现的。另外，DiscoveryClient同样在RestClient上进行了封装。

DiscoveryClient可以将资源信息缓存在本地，默认地址为~/.kube/cache和~/.kube/http-cache。
缓存可以减轻Kubernetes API Server的压力，默认10分钟同步一次，因为资源组、资源版本和资源信息变动较少。

## 源码分析

TODO