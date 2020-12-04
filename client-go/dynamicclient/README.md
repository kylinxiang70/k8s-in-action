# DynamicClient

## 简介

DynamicClient是一种动态客户端，它可以对任意Kubernetes资源进行操作，包括CRD。

与ClientSet不同，ClientSet只能操作Kubernetes自带的资源（即客户端集合内的资源）。
ClientSet需要预先实现Resource和Version的操作，其内部数据都是结构化数据（即已知道数据结构）。
而Dynamic内部实现了Unstructed，用于处理非结构化数据结构（即无法提前预支结构的数据），
这也是DynamicClient能处理CRD资源的关键。

> DynamicCliente不是类型安全的，因此处理时需要特别注意

DynamicClient处理过程将Resource转化为Unstructed结构类型。处理完整后再将Unstructed转化Resource类型。
整个过程类似于Go语言的interface{}断言转换过程。
另外，Unstructed结构类型通过`map[string]interface{}`转换。

## 源码分析

TODO