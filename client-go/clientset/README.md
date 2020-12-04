# ClientSet

## 简介
ClientSet在RestClient的基础上封装了对Resource和Version的管理方法。
每一个Resource可以理解为一个客户端，而ClientSet就相当于是多个客户端的集合，
每个Resource和Version都以函数的形式暴露给开发者。

使用RestClient需要知道GVR（Group, Version, Resource）等信息，
编码时需要知道Resource对应的Group和Version信息。

ClientSet使用起来更加便捷，一般情况下，开发者对Kubernetes进行二次开发通常使用ClientSet。

## 源码分析