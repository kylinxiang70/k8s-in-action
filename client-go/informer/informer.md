# Informer 概述

## 简介

在 kubernetes 系统中, 组件之间通过 http 协议进行通信, 
通过 informer 来做到了消息的实时性、可靠性、顺序性.

通过 informer 机制与 api-server 进行通信,
降低了了 Kubernetes 各个组件跟 Etcd 与 Kubernetes API Server 的通信压力.

## 架构设计

![informer架构](img/informer-arch.jpg)

这张图分为两部分, 黄色图标是开发者需要自行开发的部分, 而其它的部分是 client-go 已经提供的, 直接使用即可.

Informer架构设计中有多个核心组件: 

1. **Reflector**: 
   用于 Watch 指定的 Kubernetes 资源, 当 watch 的资源发生变化时, 触发变更的事件, 
   比如 Added, Updated 和 Deleted 事件, 并将资源对象存放到本地缓存 DeltaFIFO; 
2. **DeltaFIFO**: 
   拆开理解, FIFO 就是一个队列, 拥有队列基本方法(ADD, UPDATE, DELETE, LIST, POP, CLOSE 等), 
   Delta 是一个资源对象存储, 保存存储对象的消费类型, 比如 Added, Updated, Deleted, Sync 等; 
3. **Indexer**: Client-go 用来存储资源对象并自带索引功能的本地存储, 
   Reflector 从 DeltaFIFO 中将消费出来的资源对象存储到 Indexer, 
   Indexer 与 Etcd 集群中的数据完全保持一致.
   从而 client-go 可以本地读取, 减少 Kubernetes API 和 Etcd 集群的压力.