# Kubernets Version

Kubernetes的资源版本控制可以分为3中：Alpha, Beta, Stable. 
迭代顺序为：Alpha >> Beta >> Stable. 

- Alpha 版本：内部测试版本, 用于开发者内部测试, 该版本不稳定, 可能被删除, 可能存在很多缺陷和漏洞. 
默认情况, Alpha版本的功能会被禁用. 一般名称为v1alpha1、v1alpha2、v2alpha1等. 
- Beta 版本：相对稳定的版本, 经过官方和社区很多次测试, 该版本可能会有较小的改变, 但不会被删除. 
默认情况下开启. 一般名称为v1beta1, v1beta2, v2beta1等. 
- Stable 版本：正式发布版本, 该版本不会被删除. 默认情况下, 功能全部开启. 一般命名为v1、v2、v3. 



资源版本代码数据结构如下：

代码路径：vendor/k8s.io/apimachinery/pkg/apis/meta/v1/types.go

```go
// APIVersions lists the versions that are available, to allow clients to
// discover the API at /api, which is the root path of the legacy v1 API.
//
// +protobuf.options.(gogoproto.goproto_stringer)=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type APIVersions struct {
	TypeMeta `json:",inline"`
	// versions are the api versions that are available.
	Versions []string `json:"versions" protobuf:"bytes,1,rep,name=versions"`
	// a map of client CIDR to server address that is serving this group.
	// This is to help clients reach servers in the most network-efficient way possible.
	// Clients can use the appropriate server address as per the CIDR that they match.
	// In case of multiple matches, clients should use the longest matching CIDR.
	// The server returns only those CIDRs that it thinks that the client can match.
	// For example: the master will return an internal IP CIDR only, if the client reaches the server using an internal IP.
	// Server looks at X-Forwarded-For header or X-Real-Ip header or request.RemoteAddr (in that order) to get the client IP.
	ServerAddressByClientCIDRs []ServerAddressByClientCIDR `json:"serverAddressByClientCIDRs" protobuf:"bytes,2,rep,name=serverAddressByClientCIDRs"`
}
```


