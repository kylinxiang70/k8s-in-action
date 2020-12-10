# Gourp, Version, Resource

Kubernetes 系统都是围绕资源构建, 其本质上是一个资源控制系统
——注册、管理、调度并维护资源状态.

Kubernetes 将资源分组和版本化, 形成了 GVR (Group/Version/Resource) 三种核心数据结构.

- `Group`: 资源组, 在 Kubernetes API Server 中也被称为 `APIGroup`.
- `Version`: 资源版本, 在 Kubernetes API Server 中也被称为 `APIVersions`.
- `Resource`: 资源, 在 Kubernetes API Server 中也被称为 `APIResource`.
- `Kind`: 资源种类, 表述 Resource 的种类, 与 Resource 同一级别.

> Kubernetes Group/Verseions/Resource 等核心数据结构源码放在 Kubernetes 项目下的
vendor/k8s.io/apimachinery/pkg/apis/meta/v1 包中.

Kubernetes 系统中支持多个 Group, 
每个 Group 拥有多个 Version, 
每个 Version 下存在多个 Resource.
其中某些资源会拥有子资源 (SubResource), 
例如 `Deployment` 拥有 `Status` 子资源.

资源组、资源版本、资源和子资源的具体表现形式为 `Gourp/Version/Resource/SubResource`, 
例如: apps/v1/Deployment/status

资源对象 (Resource Object), 由"资源组+资源版本+资源种类组成", 
其表现形式为 `<Group>/<Version>,Kind=<kind>`,
例如: `apps/v1,Kind=Deployment`.

没有资源都有一定的操作方法, 来支持资源的 CRUD 操作.
目前 Kubernetes 支持8中操作方法: create/delete/deletecollection/get/list/patch/update/watch

每一个资源至少有两个版本：
- 内部版本 (Internal Version): 不对外暴露, 仅仅在 Kubernetes API Server 内部使用
- 外部版本 (External Version): 外部版本用于对外暴露给用户请求的接口

资源也分为两种:
- 内置资源 (Kubernetes Resource): Kubernetes 内置的资源.
- 自定义资源 (CRD, Custom Resource Definition): 自定义资源