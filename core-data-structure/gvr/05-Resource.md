# Kubernetes Resource

# Resource (资源)

Kubernetes 的本质是一个资源控制系统——管理、调度资源并维护资源的状态. 
一个资源被实例化后会表达为一个资源对象 (即 Resource Object).
所有的资源对象都是 Entity, Kubernetes 使用 Entity 来表示资源的状态. 

目前Kubernetes支持两种Entity:

- 持久性实体 (Persistent Entity):在资源对象被创建后, Kubernetes 会持久确保该资源对象存在. 
例如 `Deployment`. 
- 短暂性实体 (Ephemeral Entity):也称为非持久性实体 (Non-Persistent Entity). 
在资源对象被创建后, 如果故障或调度失败, 不会重新创建该资源对象. 例如 `Pod`. 

资源代码数据结构实例如下:

代码路径:`vendor/k8s.io/apimachinery/pkg/apis/meta/v1/types.go`

```go
// APIResource specifies the name of a resource and whether it is namespaced.
type APIResource struct {
	// name is the plural name of the resource.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// singularName is the singular name of the resource.  This allows clients to handle plural and singular opaquely.
	// The singularName is more correct for reporting status on a single item and both singular and plural are allowed
	// from the kubectl CLI interface.
	SingularName string `json:"singularName" protobuf:"bytes,6,opt,name=singularName"`
	// namespaced indicates if a resource is namespaced or not.
	Namespaced bool `json:"namespaced" protobuf:"varint,2,opt,name=namespaced"`
	// group is the preferred group of the resource.  Empty implies the group of the containing resource list.
	// For subresources, this may have a different value, for example: Scale".
	Group string `json:"group,omitempty" protobuf:"bytes,8,opt,name=group"`
	// version is the preferred version of the resource.  Empty implies the version of the containing resource list
	// For subresources, this may have a different value, for example: v1  (while inside a v1beta1 version of the core resource's group)".
	Version string `json:"version,omitempty" protobuf:"bytes,9,opt,name=version"`
	// kind is the kind for the resource  (e.g. 'Foo' is the kind for a resource 'foo')
	Kind string `json:"kind" protobuf:"bytes,3,opt,name=kind"`
	// verbs is a list of supported kube verbs  (this includes get, list, watch, create,
	// update, patch, delete, deletecollection, and proxy)
	Verbs Verbs `json:"verbs" protobuf:"bytes,4,opt,name=verbs"`
	// shortNames is a list of suggested short names of the resource.
	ShortNames []string `json:"shortNames,omitempty" protobuf:"bytes,5,rep,name=shortNames"`
	// categories is a list of the grouped resources this resource belongs to  (e.g. 'all')
	Categories []string `json:"categories,omitempty" protobuf:"bytes,7,rep,name=categories"`
	// The hash value of the storage version, the version this resource is
	// converted to when written to the data store. Value must be treated
	// as opaque by clients. Only equality comparison on the value is valid.
	// This is an alpha feature and may change or be removed in the future.
	// The field is populated by the apiserver only if the
	// StorageVersionHash feature gate is enabled.
	// This field will remain optional even if it graduates.
	// +optional
	StorageVersionHash string `json:"storageVersionHash,omitempty" protobuf:"bytes,10,opt,name=storageVersionHash"`
}
```

## 资源的内部版本和外部版本

Kuberntes 的资源定义在源码 `pkg/api` 目录下. 
同一个资源对应着两个版本, 内部版本 (Internal Version)和外部版本 (External Version). 
例子:`Deployment` 资源对象, 外部版本表现为`apps/v1`, 内部版本表现为`apps/_internal`.

### 外部版本资源对象 (External Object)

外部版本资源对象, 也成为 Versioned Object (即拥有资源版本的资源对象). 
外部版本用于暴露给用户, 比如通过Json和Yaml格式的请求. 
外部版本的资源对象通过资源版本进行标识 (**Alpha, Beta, Stable**). 

资源的外部版本定义在:`pkg/apis/<group>/<version>/`

Kubernetes 源码中, 外部版本的资源类型定义在 `vendor/k8s.io/api` 目录:

```
vendor/k8s.io/api/<gourp>/<version>/<resource file>/
```

外部资源由于需要对外暴露, 因此定义了 JSON Tag 和 Protocol Tag.

不同资源版本包在源码中的引用路径不同, 代码示例如下:

```go
corev1 "k8s.io/api/core/v1" //外部资源版本 (资源类型)
core "k8s.io/kubernetes/pkg/apis/core" //内部资源版本
k8s_api_v1 "k8s.io/kubernetes/pkg/api/core/v1" //外部资源版本 (与资源相关的函数, 例如资源转换函数)
```

### 内部版本资源对象 (Internal Object)

内部版本资源对象. 内部版本不对外暴露, 尽在 Kubernetes API Server 内部使用. 
内部版本用于多资源版本的转换, 例如v1beta1 >> v1, 其过程为v1beta1 >> internal >> v1. 
内部版本资源对象通过 `runtime.APIVersionInternal` 标识. 

内部部资源对象代码定义在下面的目录中:

```
pkg/apis/<group>/
```

---

## 资源代码定义

Kubernetes 资源代码定义在 `pkg/apis` 目录下, 
同一资源对应的内部版本和外部版本的资源代码结构并不相同. 

资源内部版本定义了所有支持的资源类型 (types.go)、
资源验证方法 (validation/validation.go)、
资源注册至资源注册表的方法 (install/install.go).
 
资源的外部版本定义了资源的转换方法 (conversion.go)、资源的默认值 (defaults.go)等. 

### 内部版本的资源代码结构

以 `Deployment` 为例, 它的内部资源版本定义在`pkg/apis/apps`目录下

```
.
├── BUILD
├── OWNERS
├── doc.go  // GoDoc文件, 定义了当前包的注释信息. 在Kubernetes资源包中, 它还担当了代码生成器的全局Tags描述文件. 
├── fuzzer
│   ├── BUILD
│   └── fuzzer.go
├── install  // 把资源组下的所有资源注册到资源注册表中
│   ├── BUILD
│   └── install.go
├── register.go  // 定义了资源组、资源版本及资源的注册信息. 
├── types.go  // 定义了当前资源组、资源版本下所支持的资源类型. 
├── v1  // 定义了资源组下拥有的资源版本的资源 (即外部版本)
│   ├── BUILD
│   ├── conversion.go
│   ├── conversion_test.go
│   ├── defaults.go
│   ├── defaults_test.go
│   ├── doc.go
│   ├── register.go
│   ├── zz_generated.conversion.go
│   └── zz_generated.defaults.go
├── v1beta1  // 定义了资源组下拥有的资源版本的资源 (即外部版本)
│   ├── BUILD
│   ├── conversion.go
│   ├── defaults.go
│   ├── defaults_test.go
│   ├── doc.go
│   ├── register.go
│   ├── zz_generated.conversion.go
│   └── zz_generated.defaults.go
├── v1beta2  // 定义了资源组下拥有的资源版本的资源 (即外部版本)
│   ├── BUILD
│   ├── conversion.go
│   ├── conversion_test.go
│   ├── defaults.go
│   ├── defaults_test.go
│   ├── doc.go
│   ├── register.go
│   ├── zz_generated.conversion.go
│   └── zz_generated.defaults.go
├── validation // 定义了资源的验证方法
│   ├── BUILD
│   ├── validation.go
│   └── validation_test.go
└── zz_generated.deepcopy.go // 定义了资源的深度复制操作, 该文件由代码生成器自动生成
```

每个Kubernetes资源目录都通过register.go代码文件定义所属的资源组和资源版本, 
内部版本资源对象通过`runtime.APIVersionInternal` (即__internal)标识. 
代码如下 (`pkg/apis/apps/register.go`):

```go
// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}
```

每一个Kubernetes资源目录, 都通过type.go代码文件定义当前资源组/资源版本下所支持的资源类型, 代码如下:

```go
type StatefulSet struct {...}
type Deployment struct {...}
```

### 外部版本的资源代码结构

以 `Deployment` 为例, 它的外部版本定义在`pkg/apis/apps{v1,v1beta1,v1beta2}`目录下, 其资源代码结构如下:
```
.
├── BUILD
├── conversion.go  //定义了资源的转换函数 (默认转换函数), 并将默认的转换函数注册到资源注册表中. 
├── conversion_test.go
├── defaults.go  // 定义了资源的默认值函数, 并将默认值函数注册到资源注册表. 
├── defaults_test.go
├── doc.go  // GoDoc文件, 定义了当前包的注释信息. 在Kubernetes资源包中, 它还担当了代码生成器的全局Tags描述文件. 
├── register.go  // 定义了资源组、资源版本及资源的注册信息. 
├── zz_generated.conversion.go  //定义了资源的转换函数 (自动生成的转换函数), 并将生成的转换函数注册到资源注册表中. 该文件由代码生成器自动生成
└── zz_generated.defaults.go  //定义了资源的默认值函数 (自动生成的默认值函数), 并将生成的转换函数注册到资源注册表中. 该文件由代码生成器自动生成 
```

外部版本资源对象通过资源版本 (Alpha,Beta,Stable)标识, 代码示例如下: (`pkg/apis/apps/v1/register.go`)

```go
// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1"}
```

### 将资源注册到资源注册表

在 Kubernetes 的资源目录中, 都拥有一个 `install/install.go` 代码文件, 
它负责将资源信息注册到资源注册表中 (scheme). 

core资源组代码示例 (`pkg/apis/core/install/install.go`)

```go
// Package install installs the v1 monolithic api, making it available as an
// option to all of the API encoding/decoding machinery.
package install

import  (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/v1"
)

func init () {
    // legacyscheme.Scheme是Kubernetes组件的全局资源注册表, Kubernetes的所有资源信息都交给资源注册表统一管理. 
	Install (legacyscheme.Scheme)
}

// Install registers the API group and adds types to a scheme
func Install (scheme *runtime.Scheme) {
	utilruntime.Must (core.AddToScheme (scheme)) // 注册core资源组内部版本资源
	utilruntime.Must (v1.AddToScheme (scheme))  // 注册core资源组外部版本资源
	utilruntime.Must (scheme.SetVersionPriority (v1.SchemeGroupVersion))  // 注册资源组的版本顺序, 如果多个资源, 潜在前面优先级越高. 
}
```

### 资源首选版本

首选版本 (Preferred Version), 也称为优选版本 (Priority Version). 
某些资源拥有多个版本, 在某些场景下不指定版本, 则使用首选版本. 
以apps资源组为例, 注册资源时会注册多个资源版本, 分别是**v1,v1beta2,v1beta1**, 
代码示例如下 (`pkg/apis/apps/install/install.go`):

```go
utilruntime.Must (scheme.SetVersionPriority (v1.SchemeGroupVersion, v1beta2.SchemeGroupVersion, v1beta1.SchemeGroupVersion))
```

在资源注册表的 `versionPriority` 结构中, 资源的首选版本如下所示:

```go
versionPriority map[string][]string // 数据结构
versionPriority "apps"[{"v1"},{"v1beta2"},{"v1beta1"}]  // 所有的外部版本的值, 不存储内部版本
```

当通过资源注册表 `scheme.PrefferedVersionAllGroups` 函数获取所有资源组下的首选版本时, 
将位于前面的资源版本作为首选版本, 代码如下所示:

```go
// PreferredVersionAllGroups returns the most preferred version for every group.
// group ordering is random.
func  (s *Scheme) PreferredVersionAllGroups () []schema.GroupVersion {
	ret := []schema.GroupVersion{}
	for group, versions := range s.versionPriority {
		for _, version := range versions {
			ret = append (ret, schema.GroupVersion{Group: group, Version: version})
			break  // 循环一次就break, 所以支取versions数组的第一个元素
		}
	}
	for _, observedVersion := range s.observedVersions {
		found := false
		for _, existing := range ret {
			if existing.Group == observedVersion.Group {
				found = true
				break
			}
		}
		if !found {
			ret = append (ret, observedVersion)
		}
	}

	return ret
}
```

除了 `scheme.PreferredVersionAllGroups` , 还有两个函数与获取资源版本的顺序有关:

- 获取所有资源的所有资源版本, 按优先级顺序返回

```go
// PrioritizedVersionsAllGroups returns all known versions in their priority order.  Groups are random, but
// versions for a single group are prioritized
func  (s *Scheme) PrioritizedVersionsAllGroups () []schema.GroupVersion {
	ret := []schema.GroupVersion{}
	for group, versions := range s.versionPriority {
		for _, version := range versions {
			ret = append (ret, schema.GroupVersion{Group: group, Version: version})
		}
	}
	for _, observedVersion := range s.observedVersions {
		found := false
		for _, existing := range ret {
			if existing == observedVersion {
				found = true
				break
			}
		}
		if !found {
			ret = append (ret, observedVersion)
		}
	}
	return ret
}
```

- 获取指定资源组的资源版本, 按照优先级顺序返回

```go
// PrioritizedVersionsForGroup returns versions for a single group in priority order
func  (s *Scheme) PrioritizedVersionsForGroup (group string) []schema.GroupVersion {
	ret := []schema.GroupVersion{}
	for _, version := range s.versionPriority[group] {
		ret = append (ret, schema.GroupVersion{Group: group, Version: version})
	}
	for _, observedVersion := range s.observedVersions {
		if observedVersion.Group != group {
			continue
		}
		found := false
		for _, existing := range ret {
			if existing == observedVersion {
				found = true
				break
			}
		}
		if !found {
			ret = append (ret, observedVersion)
		}
	}

	return ret
}
```

### 资源操作方法

Kubernetes 支持的操作有8种, 分别为:`create`、`delete`、`deletecollection`、`get`、`list`、`patch`、`update`、`watch`. 

这些操作可以分为4大类:

- 创建:`create`
- 删除:`delete`、`deletecollection`
- 更新:`patch`、`update`
- 查询:`get`、`list`、`watch`

资源操作通过通过 `metav1.Verbs` 数据结构进行描述, 
代码示例如下 (`vendor/k8s.io/apimachinery/pkg/apis/meta/v1/types.go`):

```go
type Verbs []string

func  (vs Verbs) String () string {
	return fmt.Sprintf ("%v", []string (vs))
}
```

资源操作方法都是针对存储 (`Storage`)进行操作, 
`vendor/k8s.io/apiserver/pkg/registry/`目录定义了资源对象拥有的操作类型. 
每种操作方法对应一个操作方法接口 (`Interface`),如下表所示:

| 操作方法 (Verbs) | 操作方法接口 (Interface) |  |
| --- | --- | --- |
| create | rest.Creater | 资源对象创建接口 |
| delete | rest.GracefulDeleter | 资源对象删除接口 (单个资源对象) |
| deletecollection | rest.CollectionDeleter | 资源对象删除接口 (多个资源对象) |
| update | rest.Updater | 资源对象更新接口 (完整资源对象的更新) |
| patch | rest.Patcher | 资源对象更新接口 (局部资源对象的更新) |
| get | rest.Getter | 资源对象获取接口 (单个资源对象) |
| list | rest.Lister | 资源对象获取接口 (多个资源对象) |
| watch | rest.Watcher | 资源对象监控接口 |

如果某个资源对象在存储 (`Storage` 上实现了)某接口, 那么该资源同时就拥有了相关的操作方法. 
相关接口定义如下 (`vendor/k8s.io/apiserver/pkg/registry/rest/rest.go`),
该文件中定义了很多接口, 下面以 `Creater` 为例:

```go
...
// Creater is an object that can create an instance of a RESTful object.
type Creater interface {
	// New returns an empty object that can be used with Create after request data has been put into it.
	// This object must be a pointer type for use with codec.DecodeInto ([]byte, runtime.Object)
	New () runtime.Object

	// Create creates a new version of a resource.
	Create (ctx context.Context, obj runtime.Object, createValidation ValidateObjectFunc, options *metav1.CreateOptions)  (runtime.Object, error)
}
...
```

以 `Pod` 对象为例, `Pod` 资源对象的存储 (`Storage`)实现了以上接口的方法, 
Pod资源对象继承了`genericregistry.Store`, 该对象可以管理存储 (`Storage`)的增删改查操作, 
代码示例如下

代码路径:`pkg/registry/core/pod/storage/storage.go`:

```go
type PodStorage struct {
    Pod  *REST
    ...
}
type REST struct {
    *genericregistry.Store
    ...
}
```

代码路径:`vendor/k8s.io/apiserver/pkg/registry/generic/registry/store.go`

```go
func  (e *Store) Create (ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions)  (runtime.Object, error) {
    ...
}

func  (e *Store) Get (ctx context.Context, name string, options *metav1.GetOptions)  (runtime.Object, error) {
    ...
}
```

下面以 `pod/logs` 子资源为例, 该资源只实现了get操作方法, 代码示例如下:

代码路径:`pkg/registry/core/pod/storage/storage.go`

```go
type PodStorage struct {
	...
	Log                 *podrest.LogREST
	...
}
```

代码路径:`pkg/registry/core/pod/rest/log.go`

```go
func  (r *LogREST) Get (ctx context.Context, name string, opts runtime.Object)  (runtime.Object, error) {
    ...
}
```

### 资源与命名空间

在 `Kubernetes` 中, 大部分资源都属于某些命名空间, 但并不是所有资源都有属于某个命名空间, 
比如 `Node` 资源对象. 
可以通过`ObjectMeta.Namespace`查看一个资源对象所属的命名空间. 

```
-------------------             ------------------
|      Pod        |             |   ObjectMata   |
-------------------             ------------------
|metav1.TypeMeta  |     ----->  |      ...       |
|metav1.ObjectMeta|             |Namespace string|
|Spec PodSpec     |             |      ...       |
|Status PodStatus |             ------------------ 
-------------------
```

查看存在于和不存在于命名空间中的资源对象

```
kubectl api-resources --namespaced=true
kubectl api-resources --namespaced=false
```
