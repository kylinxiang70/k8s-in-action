# DynamicClient

## 简介

DynamicClient 是一种动态客户端, 它可以对任意 Kubernetes 资源进行操作, 包括 CRD.

与 ClientSet 不同, ClientSet 只能操作 Kubernetes 自带的资源（即客户端集合内的资源）.
ClientSet 需要预先实现 Resource 和 Version 的操作, 其内部数据都是结构化数据（即已知道数据结构）.
而 Dynamic 内部实现了 Unstructed, 用于处理非结构化数据结构（即无法提前预支结构的数据）, 
这也是 DynamicClient 能处理 CRD 资源的关键.

> DynamicCliente 不是类型安全的, 因此处理时需要特别注意

DynamicClient 处理过程将 Resource 转化为 Unstructed 结构类型.
处理完整后再将 Unstructed 转化 Resource 类型.
整个过程类似于 Go 语言的 `interface{}` 断言转换过程.
另外, Unstructed 结构类型通过 `map[string]interface{}` 转换.

## 源码分析

TODO