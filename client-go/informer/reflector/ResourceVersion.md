# Resource Version

> https://kubernetes.io/docs/reference/using-api/api-concepts/#resource-versions

资源版本（Resource Version）是标识服务器对象内部版本的字符串。客户端可以使用资源版本来确定对象何时更改，或在获取、列出和查看资源时表示数据一致性要求。资源对于客户端是可见的，但是客户端不能修改`ResourceVersion`。

## Metadata 中的 ResourceVersion

Client 从资源中发现资源版本，包括 watch 事件中的资源，并且列举所有从服务器返回的响应：

- [v1.meta/ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) - 一个资源实例的 `metadata.resourceVersion` 字段标识该实例上一次修改的资源版本。

- [v1.meta/ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#listmeta-v1-meta) - 一个资源集合（比如一个资源列表)的`metadata.resourceVersion`标识整个资源列表结构的资源版本。

一个资源只能包含{ObjectMeta, ListMeta}中的一个。

## ResourceVersion 相关参数

Get/List/Watch 操作支持 `resourceVersion` 参数

该参数的确切含义因操作和`resourceVersion`的值而异。

对于 Get 和 List，`resourceVersion` 的语义如下：

GET:

- ResourceVersion unsert: Most recent 
- resourceVersion="0": any 

- resourceVersion="{value other than 0}": NotOlderThan（不低于ResourceVersion的值）

List:

v1.19+版本的API Server支持`resourceVersionMatch`参数，这个参数决定如何将resourceVersion应用于list。

强烈建议为设置了resourceVersion的列表调用设置resourceVersionMatch。

如果 resourceVersion没有设置，则不允许设置resourceVersionMatch。

为了向后兼容，客户端必须容忍无服务器忽略resourceVersionMatch：

- 当使用 resourceVersionMatch=NotOlderThan并且设置了limit，客户端则必须处理HTTP 410 "Gone"响应。例如，客户端可能会使用较新的resourceVersion重试，或回退到resourceVersion=""。
- 当使用resourceVersionMatch=Exact并且没有设置limit，客户端必须检验响应中ListMeta的resourceVersion是否和请求的resourceVersion相等，并且处理不相等的情况。比如，客户端可能会回退到设置了limit的请求。

除非您有强烈的一致性要求，否则使用resourceVersionMatch=NotOolderThan和已知的resourceVersion是最好的，因为它可以获得更好的群集性能和可扩展性，而不是不设置resourceVersion和resourceVersionMatch，因为这会使用quorum read。

| resourceVersionMatch param | paging params              | resourceVersion unset | resourceVersion="0"                      | resourceVersion="{value other than 0}" |
| :------------------------- | :------------------------- | --------------------- | ---------------------------------------- | -------------------------------------- |
| unset                      | limit unset                | Most Recent           | Any                                      | Not older than                         |
| unset                      | limit=<n>, continue unset  | Most Recent           | Any                                      | Exact                                  |
| unset                      | limit=<n>,continue=<token> | Continue Token,Exact  | Invalid,treated as Continue Token, Exact | Invalid, HTTP 400 Bad Request          |
| Exact[1]                   | limit unset                | Invalid               | Invalid                                  | Exact                                  |
| Exact[1]                   | limit=<n>,continue unset   | Invalid               | Invalid                                  | Exact                                  |
| NotOlderThan[1]            | limit unset                | Invalid               | Any                                      | Not older than                         |
| NotOlderThan[1]            | limit=<n>,continue unset   | Invalid               | Any                                      | Not older than                         |

脚注：如果server不支持resourceVersionMatch参数，则视为unset。

get和list请求的语义如下：

- Most Recent：返回最新resourceVersion的数据。返回的数据必须是一致的（比如etcd使用quorum read）。
- Any：返回任意版本resourceVersion的数据。首选版本为最新的可用资源版本，但不需要强一致性，可以提供任何resource version的数据。因为网络分区和缓存过时，请求可能返回一个客户端已经观察到的resourceVersion过时的数据。如果客户端不能容忍这种情况，则不能使用这种语义。
- Not older than：返回的数据至少和给定的resourceVersion一样新。首选最新的可用的数据，但是任意不早于给定resourceVersion的数据都可能被返回。如果服务端支持resourceVersionMatch参数，对于list请求，可以保证ListMeta不会早于请求的resourceVersion，但是并不保证ObjectMeta中的resourceVersion。resourceVersion跟踪对象上次更新的时间，而不是对象在服务时的最新程度。
- Exact：提供确定版本resourceVersion的数据。如果提供的resourceVersion不可用，服务端将返回HTTP 410“Gone”响应。如果服务端支持resourceVersionMatch参数，当发送list请求时，保证ListMeta中的resourceVersion与请求的resourceVersion相同。但不保证列表项的ObjectMeta中的resourceVersion，因为ObjectMeta.resourceVersion跟踪对象上次更新的时间，而不是对象在服务时的最新程度。
- Continue Token, Exact：返回初始分页请求中的resourceVersion的数据。当初始的分页请求完成之后，针对之后所有的分页请求，Continue Token负责跟踪初始提供的resourceVersion。

Watch请求语义如下：

| resourceVersion unset              | resourceVersion="0"        | resourceVersion="{value other than 0}" |
| ---------------------------------- | -------------------------- | -------------------------------------- |
| Get State and Start at Most Recent | Get State and Start at Any | Start at Exact                         |

watch语义如下：

- Get State and Start at Most Recent：从最新的resourceVersion开始watch，必须是强一致性（比如etcd的quorum read）。为了建立初始状态，从所有开始watch时就在存在的资源的"Added"事件开始watch，所有资源变化的watch事件都发生在开始watch时的resourceVersion之后。
- Get State and Start at Any：警告：如果watch通过这种方式初始化可能导致返回任意过时数据。在使用前，请重新审查这个语义，最好选择其他符合的语义。从任意的resourceVersion开始Watch，如果最新resourceVersion的数据可用，则优先从最新的resourceVersion开始，但是不强制，任何数据都是允许返回的。客户端可能从一个已经观察到的过时的资源版本开始watch，特别是在高可用的配置下，容易出现分区和缓存过时的情况。如果不能容忍这种情况，则不应该使用这种语义watch。为了建立初始状态，从所有开始watch时就在存在的资源的"Added"事件开始watch，所有资源变化的watch事件都发生在开始watch时的resourceVersion之后。
- Start at Exact：从一个确定的resourceVersion开始watch。所有资源变更的watch事件都从指定的resourceVersion开始。与“Get State and Start at Most Recent" 和 "Get State and Start at Any" 从synthetic "Added"事件给定的resourceVersion开始不同。客户端假定已经有了初始的状态，并且从客户端提供的resourceVersion开始watch。

## "410 Gone" response

服务端不需要为所有较旧的resourceVersion提供服务，如果客户端请求的resourceVersion早于服务器保留的资源版本，服务器可能会返回HTTP 410 (Gone)状态代码。

## Unavailable resource version

服务端不会处理未识别的resourceVersion。list和get未识别resourceVersion的请求可能会短暂等待resourceVersion变为可用，如果指定的resourceVersion在合理的时间内无法变为可用，则超时并返回状态码504（Gateway timeout），并可能使用retry-after响应头进行响应，指示客户端在重试请求之前应等待多少秒。目前，kube-apiserver还使用“Too large resource versions”消息标识这些响应。对无法识别的资源版本的watch请求可能会无限期等待（直到请求超时）。

# 高效感知变化

要使客户端能够构建群集当前状态的模型，所有Kubernetes对象资源类型都需要支持list和watch。每一个Kubernetes对象都有一个resourceVersion字段用来存储对应数据库中资源的版本。当获取资源集合时（包括集群范围和命名空间范围），来自服务端的响应将包含一个resourceVersion值，该值可用于启动对该服务端的watch。服务器将返回在提供的resourceVersion之后发生的所有更改（创建、删除和更新）。这允许客户端获取当前状态，然后在不丢失任何更新的情况下watch更改。如果客户端watch断开，他们可以从上次返回的resourceVersion重新启动新的watch，或者执行新的collection request并重新开始。

给定的Kubernetes服务器将仅在有限的时间内保留更改的历史列表。默认情况下，使用etcd3的群集保留过去5分钟内的更改。当请求的watch操作因该资源的历史版本不可用而失败时，客户端必须通过识别状态代码410 Gone、清除其本地缓存、执行列表操作以及从该新列表操作返回的resourceVersion启动监视来处理此案例。大多数客户端库为此逻辑提供了某种形式的标准工具。（在Go中，这称为Reflector，位于k8s.io/client-go/cache包中。）

例子：

1. list 所有的test namespace下的 pod

```JSON
GET /api/v1/namespaces/test/pods
---
200 OK
Content-Type: application/json

{
  "kind": "PodList",
  "apiVersion": "v1",
  "metadata": {"resourceVersion":"10245"},
  "items": [...]
}
```

2. 从10245 resourceVersion开始，以单独的JSON对象的形式，接收任意创建、删和修改的通知

```json
GET /api/v1/namespaces/test/pods?watch=1&resourceVersion=10245
---
200 OK
Transfer-Encoding: chunked
Content-Type: application/json

{
  "type": "ADDED",
  "object": {"kind": "Pod", "apiVersion": "v1", "metadata": {"resourceVersion": "10596", ...}, ...}
}
{
  "type": "MODIFIED",
  "object": {"kind": "Pod", "apiVersion": "v1", "metadata": {"resourceVersion": "11020", ...}, ...}
}
...
```



## Watch bookmarks

为了减轻短历史窗口的影响，k8s引入了bookmark watch事件。这是一种特殊的事件，用于标记客户端请求的给定resourceVersion的所有更改都已发送。该事件中，返回对象是客户端请求所指定的类型，但是只设置了resourceVesion字段。

例子：

```json
GET /api/v1/namespaces/test/pods?watch=1&resourceVersion=10245&allowWatchBookmarks=true
---
200 OK
Transfer-Encoding: chunked
Content-Type: application/json

{
  "type": "ADDED",
  "object": {"kind": "Pod", "apiVersion": "v1", "metadata": {"resourceVersion": "10596", ...}, ...}
}
...
{
  "type": "BOOKMARK",
  "object": {"kind": "Pod", "apiVersion": "v1", "metadata": {"resourceVersion": "12746"} }
}
```

Bookmark事件可以使用allowWatchBookmarks=true选项在特定watch请求中指定，但是客户端不能假设会在指定间隔时间返回，或者假定服务端将会返回bookmark类型的事件。

