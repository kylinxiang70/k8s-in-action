# ClientSet

## 简介
ClientSet 在 RestClient 的基础上封装了对 `Resource` 和 `Version` 的管理方法。
每一个 `Resource` 可以理解为一个客户端，而 ClientSet 就相当于是多个客户端的集合，
每个 `Resource` 和 `Version` 都以函数的形式暴露给开发者。

使用 RestClien 需要知道 GVR（Group, Version, Resource）等信息，
编码时需要知道 `Resource` 对应的 `Group` 和 `Version` 信息。

ClientSet 使用起来更加便捷，一般情况下，开发者对 Kubernetes 进行二次开发通常使用 ClientSet。

## 源码分析
TODO