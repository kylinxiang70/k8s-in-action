# runtime.Object类型基石

Kubernetes Runtime 在 vendor/k8s.io/apimachinery/pkg/runtime 中实现, 
它提供了通用资源类型 `runtime.Object`.
`runtime.Object` 是Kubernetes 类型系统的基石.
资源对象（Resource Object）都有一个共同的结构 `runtime.Object`.
`runtime.Object` 被设计为接口, 作为资源对象的通用资源对象.
```
Pod 资源对象可以和 runtime.Object 相互转化   Deployment 资源对象可以和 runtime.Object 相互转化
---------------              ------------------                 --------------------
| &core.Pod{} |    <---->    | runtime.Object |    <------>     | &apps.Deployment |
---------------              ------------------                 --------------------
```

`runtime.Object` 结构如下：

代码路径：vendor/k8s.io/apimachinery/pkg/runtime/interfaces.go

```go
// Object interface must be supported by all API types registered with Scheme. Since objects in a scheme are
// expected to be serialized to the wire, the interface an Object must provide to the Scheme allows
// serializers to set the kind, version, and group the object is represented as. An Object may choose
// to return a no-op ObjectKindAccessor in cases where it is not expected to be serialized.
type Object interface {
	GetObjectKind() schema.ObjectKind
	DeepCopyObject() Object
}
```

代码路径：vendor/k8s.io/apimachinery/pkg/runtime/schema/interfaces.go

```go
// All objects that are serialized from a Scheme encode their type information. This interface is used
// by serialization to set type information from the Scheme onto the serialized version of an object.
// For objects that cannot be serialized or have unique requirements, this interface may be a no-op.
type ObjectKind interface {
	// SetGroupVersionKind sets or clears the intended serialized kind of an object. Passing kind nil
	// should clear the current setting.
	SetGroupVersionKind(kind GroupVersionKind)
	// GroupVersionKind returns the stored group, version, and kind of an object, or an empty struct
	// if the object does not expose or provide these fields.
	GroupVersionKind() GroupVersionKind
}
```

`runtime.Object` 提供了两个方法

- `GetObjectKind() schema.ObjectKind`：用于设置并返回GroupVersionKind
- `DeepCopyObject() Object`：用于深拷贝当前资源对象并返回

>  深拷贝将数据结构重新克隆一份, 不与原始对象共享任何内容.



如何确认一个资源队形是否可以转换为 `runtime.Object` 通用资源对象？
需要确认资源对象是否实现了 `GetObjectKind` 和 `DeepCopyObject` 方法.
Kubernetes 的每一个资源对象都嵌入了`metav1.TypeMeta` 类型, 
`metav1.TypeMeta` 实现了 `GetObjectKind` 方法, 所有资源对象都实现了该方法.
Kubernetes 的每一个资源对象都实现了 `DeepCodeObject` 方法, 
该方法一般被定义在 zz_generated.deepcopy.go 文件中.


Kubernetes 的任意资源对象可以通过 `runtime.Object` 存储他的类型, 并允许深度复制操作.
下面的代码示例将资源对象转换为 `runtime.Object`, 然后再转换回资源对象.

```go
package main

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/apis/core"
)

func main() {

	pod := &core.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"foo": "foo"},
		},
	}

	obj := runtime.Object(pod)

	pod2, ok := obj.(*core.Pod)

	if !ok {
		panic("unexpected")
	}

	if !reflect.DeepEqual(pod, pod2) {
		panic("unexpected")
	}
}
```

---

导入 k8s.io/kubernetes 依赖时, 会遇到类似下面的错误

```go
k8s.io/api@v0.0.0: reading k8s.io/api/go.mod at revision v0.0.0: unknown revision v0.0.0
```

使用replace命令解决

```go
replace (
    k8s.io/api => k8s.io/api v0.0.0-20190620084959-7cf5895f2711
    k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190620085554-14e95df34f1f
    k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
    k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190620085212-47dc9a115b18
    k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190620085706-2090e6d8f84c
    k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
    k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190620090043-8301c0bda1f0
    k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190620090013-c9a0fc045dc1
    k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190612205613-18da4a14b22b
    k8s.io/component-base => k8s.io/component-base v0.0.0-20190620085130-185d68e6e6ea
    k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190531030430-6117653b35f1
    k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20190620090116-299a7b270edc
    k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190620085325-f29e2b4a4f84
    k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20190620085942-b7f18460b210
    k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20190620085809-589f994ddf7f
    k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20190620085912-4acac5405ec6
    k8s.io/kubectl => k8s.io/kubectl v0.0.0-20201008135616-e95e378e5972
    k8s.io/kubelet => k8s.io/kubelet v0.0.0-20190620085838-f1cb295a73c9
    k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20190620090156-2138f2c9de18
    k8s.io/metrics => k8s.io/metrics v0.0.0-20190620085625-3b22d835f165
    k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20190620085408-1aef9010884e
)
```


