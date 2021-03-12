# Resource Version

资源版本（Resource Version）是标识服务器对象内部版本的字符串。客户端可以使用资源版本来确定对象何时更改，或在获取、列出和查看资源时表示数据一致性要求。资源对于客户端是可见的，客户端必须在不修改资源版本的情况下返回给服务器。例如，客户端不得假定资源版本是数字版本，并且只能比较两个资源版本来判断资源是否相等（即不得比较资源版本的大小）。

## Metadata 中的 ResourceVersion

Client 从资源中发现资源版本，包括 watch 事件中的资源，并且列举所有从服务器返回的响应：

- [v1.meta/ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) - 一个资源实例的 `metadata.resourceVersion` 字段标识该实例上一次修改的资源版本。

- [v1.meta/ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#listmeta-v1-meta) - 一个资源集合（比如一个资源列表)的`metadata.resourceVersion`标识整个资源列表结构的资源版本。

一个资源只能包含{ObjectMeta, ListMeta}中的一个。

## ResourceVersion 的参数

Get/List/Watch 操作支持 `resourceVersion` 参数

该参数的确切含义因操作和`resourceVersion`的值而异。

对于 Get 和 List，`resourceVersion` 的语义如下：

GET:

- ResourceVersion unset: Most Recent
- resourceVersion="0": Any
- resourceVersion="{value other than 0}": Not older than

List:
