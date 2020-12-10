# Kubernetes Group

Group（资源组）, 在 Kubernetes API Server 中也称为 API Group. 
资源按照不同功能划分到不同的资源组. 

资源组特点如下: 

- 允许单独启用/进制某个资源组（资源也可以）
- 支持资源组中拥有不同的资源版本, 方便迭代升级
- 支持同名的资源（即 Kind）位于不同的资源组内
- 资源组和资源版本通过 Kubernetes API Server 对外暴露, 
可以通过HTTP协议进行交互并通过动态客户端（即 DynamicClient）进行资源发现
- 支持 CRD 自定义资源扩展
- 使用 kubectl 可以不同写资源组名称



资源组数据结构如下: 

代码路径: vendor/k8s.io/apimachinery/pkg/apis/meta/v1/types.go

```go
// APIGroup contains the name, the supported versions, and the preferred version
// of a group.
type APIGroup struct {
	TypeMeta `json:",inline"`
	// name is the name of the group.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// versions are the versions supported in this group.
	Versions []GroupVersionForDiscovery `json:"versions" protobuf:"bytes,2,rep,name=versions"`
	// preferredVersion is the version preferred by the API server, which
	// probably is the storage version.
	// +optional
	PreferredVersion GroupVersionForDiscovery `json:"preferredVersion,omitempty" protobuf:"bytes,3,opt,name=preferredVersion"`
	// a map of client CIDR to server address that is serving this group.
	// This is to help clients reach servers in the most network-efficient way possible.
	// Clients can use the appropriate server address as per the CIDR that they match.
	// In case of multiple matches, clients should use the longest matching CIDR.
	// The server returns only those CIDRs that it thinks that the client can match.
	// For example: the master will return an internal IP CIDR only, if the client reaches the server using an internal IP.
	// Server looks at X-Forwarded-For header or X-Real-Ip header or request.RemoteAddr (in that order) to get the client IP.
	// +optional
	ServerAddressByClientCIDRs []ServerAddressByClientCIDR `json:"serverAddressByClientCIDRs,omitempty" protobuf:"bytes,4,rep,name=serverAddressByClientCIDRs"`
}
```


Kubernetes 系统中支持两种资源组: 

- 拥有组名的资源组: 表现形式为 `<group>/<version>/<resource>`, 例如 `apps/v1/deployments`. 
   - http api: http://localhost:8080/apis/apps/v1/deployments
- 没有组名的资源组: 被称为 `Core Groups`（即核心资源组）或 `Legancy Groups`. 
其表现形式为`<version>/<resource>`, 例如 `v1/pods`. 
   - http api: http://localhost:8080/apis/v1/pods
