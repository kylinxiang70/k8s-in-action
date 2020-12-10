# Scheme资源注册表

Kubernetes 拥有众多资源, 每一种资源就是一个资源类型, 
这些资源需要有统一的注册、存储、查询、管理机制.
目前 Kubernetes 系统中所有的资源类型都已经注册到 Scheme 资源注册表中, 
其实一个内存型的资源注册表, 拥有如下特点.

- 支持注册多种资源类型, 包括内部版本和外部版本
- 支持多种版本转换机制
- 支持不同资源的序列化和反序列化机制



`Scheme` 支持两种资源类型（Type）的注册, 分别是 `UnversionedType` 和 `KnownType` 资源类型：

- `UnversionedType`：无版本资源类型, 早期 Kubernetes 系统概念, 
主要应用于没有版本的资源对象, 不需要进行转换. 在目前的 Kubernetes 发行版本中, 
无版本类型已经被弱化, 几乎所有的资源对象都拥有版本. 在 metav1 元数据中还有部分类型既属于 `meta.k8s.io/v1`
又属于 `UnversionedType`. 例如：`metav1.Status`、`metav1.APIVersion`、`metav1.APIGroupList`、`metav1.APIGroup`、`metav1.APIResourceList`.
   - 在资源注册表中, 通过 `scheme.AddKnownTypes` 方法进行注册
- KnownType：是目前 Kubernetes 最常用的资源类型, 也称为“拥有版本的资源类型”
   - 在资源注册表中, 通过 `scheme.AddKnownTypes` 方法进行注册



## Scheme资源注册表数据结构

`Scheme` 资源注册表的数据结构主要由4个 `map` 组成, 如下代码所示：

```go
type Scheme struct {
	// versionMap allows one to figure out the go type of an object with
	// the given version and name.
    // 存储 GVK 与 Type 的映射关系
	gvkToType map[schema.GroupVersionKind]reflect.Type

	// typeToGroupVersion allows one to find metadata for a given go object.
	// The reflect.Type we index by should *not* be a pointer.
    // 存储 Type 与 GVK 的映射关系, 一个 Type 会对应一个或多个 GVK
	typeToGVK map[reflect.Type][]schema.GroupVersionKind

	// unversionedTypes are transformed without conversion in ConvertToVersion.
    // 存储 UnversionedType 与 GVK 的映射关系
	unversionedTypes map[reflect.Type]schema.GroupVersionKind

	// unversionedKinds are the names of kinds that can be created in the context of any group
	// or version
	// TODO: resolve the status of unversioned types.
    // 存储 Kind (资源种类）名称与 UnversionedType 的映射关系
	unversionedKinds map[string]reflect.Type
    
    ...
}
```

Scheme 资源注册表通过 `map` 实现映射关系, 高效实现了正向和反向检索, 
从 `Scheme` 资源注册表中检索某个 GVK 的 Type , 时间复杂度为 O(1).


`Scheme` 资源注册表在 Kubernetes 系统中属于非常核心的数据结构, 
直接阅读源码十分晦涩, 通过下面的代码理解 `Scheme` 资源注册表.

```go
package main

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	// KnownType External
	coreGV := schema.GroupVersion{Group: "", Version: "v1"}
	extensionsGV := schema.GroupVersion{Group: "extensions", Version: "v1beta1"}

	// KnownType internal
	coreInternalGV := schema.GroupVersion{Group: "", Version: runtime.APIVersionInternal}

	// UnversionedType
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(coreGV, &corev1.Pod{})
	scheme.AddKnownTypes(extensionsGV, &appsv1.DaemonSet{})
	scheme.AddKnownTypes(coreInternalGV, &corev1.Pod{})
	scheme.AddUnversionedTypes(unversioned, &metav1.Status{})
}
```

上述代码中, 首先定义了两种类型的 `GV（Group, Version）`, `KnownType` 类型有 `coreGV`, `extensionGV`、
`coreInternalGV` 对象, 其中 `coreInternalGV` 属于内部版本（即 `runtime.APIInternalVersion`）, 
而 `UnversionedType` 类型有 `Unversioned` 对象.
`AddKnownTypes` 和 `AddUnversionedTypes` 分别将 `KnownTypes` 和 `UnversionedTypes` 添加到 `Scheme`.


在上面的代码示例中, 我们注入了 `Pod`、`DeamonSet`、`Pod（内部版本）` 及 `Status（无版本资源类型）` 对象, 得到以下的映射关系：

```
gvkToType            ------------------------------------------------------
                     |        map[schema.GroupVersionKind]reflect.Type    |
                     ------------------------------------------------------
                     |                       /v1,Kind=Pod : v1.Pod        |
                     |  extensions/v1beta1,Kind=DeamonSet : v1.DeamonSet  |
                     |               /_internal,Kind=Pod, : v1.Pod        |
                     |                    /v1,Kind=Status : v1.Status     |
                     ------------------------------------------------------


typeToGVK            ------------------------------------------------------
                     |        map[reflect.Type]schema.GroupVersionKind    |
                     ------------------------------------------------------
                     |    v1.Status : [/v1,Kind=Status]                   |
                     |       v1.Pod : [/v1,Kind=Pod, /_internal,Kind=Pod] |
                     | v1.DeamonSet : [extensions/v1beta1,Kind=DeamonSet] |  
                     ------------------------------------------------------
                     
                     
unversionedTypes     ------------------------------------------------------
                     |        map[schema.GroupVersionKind]reflect.Type    |
                     ------------------------------------------------------
                     |           v1.Status: /v1,Kind=Status               |
                     ------------------------------------------------------
                     
                     
unversionedKinds     ------------------------------------------------------
                     |        map[schema.GroupVersionKind]reflect.Type    |
                     ------------------------------------------------------
                     |           v1.Status: /v1,Kind=Status               |
                     ------------------------------------------------------
```

`GVK` 在 `Scheme` 中以 `<group>/<version>,Kind=<kind>` 的形式存在, 
对于 `Kind` 字段, 如果在注册时不指定该字段的名称, 那么默认使用类型的名称, 
例如 `corev1.Pod` 类型, 通过reflect机制获取资源类型的名称, 那么它的资源种类 `Kind=Pod`.
资源类型在 `Scheme` 资源注册表中以 Go `reflect.Type` 形式存在.


另外, `UnversionedType` 类型的对象在通过` scheme.AddUnversionedTypes` 方法注册时, 
会同时存在于4个map中, 代码示例如下：

代码路径：vendor/k8s.io/apimachinery/pkg/runtime/scheme.go

```go
func (s *Scheme) AddUnversionedTypes(version schema.GroupVersion, types ...Object) {
	s.addObservedVersion(version)
	s.AddKnownTypes(version, types...) // 加入前两个Map
	for _, obj := range types {
		t := reflect.TypeOf(obj).Elem()
		gvk := version.WithKind(t.Name())
		s.unversionedTypes[t] = gvk  // 加入第三个Map
		if old, ok := s.unversionedKinds[gvk.Kind]; ok && t != old {
			panic(fmt.Sprintf("%v.%v has already been registered as unversioned kind %q - kind name must be unique in scheme %q", old.PkgPath(), old.Name(), gvk, s.schemeName))
		}
		s.unversionedKinds[gvk.Kind] = t  // 加入第四个Map
	}
}
```
## 资源注册表注册方法
不同资源的注册方法不同, 如下所示：

- `scheme.AddUnversionedTypes`：注册 `UnversionedTypes` 资源类型
- `scheme.AddKnownTypes`：注册 `KnownTypes` 资源类型
- `scheme.AddKnownTypesWithName`：注册 `KnownTypes` 资源类型, 须指定资源的Kind资源种类名称



```go
func (s *Scheme) AddKnownTypes(gv schema.GroupVersion, types ...Object) {
	s.addObservedVersion(gv)
	for _, obj := range types {
		t := reflect.TypeOf(obj)
		if t.Kind() != reflect.Ptr {
			panic("All types must be pointers to structs.")
		}
		t = t.Elem()
		s.AddKnownTypeWithName(gv.WithKind(t.Name()), obj)
	}
}
```


## 资源注册表查询方法

在运行过程中, Kube-apiserver 组件常对 `Scheme` 资源注册表进行查询, 他提供了如下方法：

- `scheme.KnownTypes`：查询注册表中指定GV下的资源类型

```go
// KnownTypes returns the types known for the given version.
func (s *Scheme) KnownTypes(gv schema.GroupVersion) map[string]reflect.Type {
	types := make(map[string]reflect.Type)
	for gvk, t := range s.gvkToType {
		if gv != gvk.GroupVersion() {
			continue
		}

		types[gvk.Kind] = t
	}
	return types
}
```

- `scheme.AllKnownTypes`：查询所有 `GVK` 下的资源类型

```go
// AllKnownTypes returns the all known types.
func (s *Scheme) AllKnownTypes() map[schema.GroupVersionKind]reflect.Type {
	return s.gvkToType
}

```

- `scheme.ObjectKinds`：查询资源对象对应的 `GVK`, 一个资源对象可能存在多个 `GVK`

```go
// ObjectKinds returns all possible group,version,kind of the go object, true if the
// object is considered unversioned, or an error if it's not a pointer or is unregistered.
func (s *Scheme) ObjectKinds(obj Object) ([]schema.GroupVersionKind, bool, error) {
	// Unstructured objects are always considered to have their declared GVK
	if _, ok := obj.(Unstructured); ok {
		// we require that the GVK be populated in order to recognize the object
		gvk := obj.GetObjectKind().GroupVersionKind()
		if len(gvk.Kind) == 0 {
			return nil, false, NewMissingKindErr("unstructured object has no kind")
		}
		if len(gvk.Version) == 0 {
			return nil, false, NewMissingVersionErr("unstructured object has no version")
		}
		return []schema.GroupVersionKind{gvk}, false, nil
	}

	v, err := conversion.EnforcePtr(obj)
	if err != nil {
		return nil, false, err
	}
	t := v.Type()

	gvks, ok := s.typeToGVK[t]
	if !ok {
		return nil, false, NewNotRegisteredErrForType(s.schemeName, t)
	}
	_, unversionedType := s.unversionedTypes[t]

	return gvks, unversionedType, nil
}
```

- `scheme.New`：查询 `GVK` 所对应的资源对象

```go
// New returns a new API object of the given version and name, or an error if it hasn't
// been registered. The version and kind fields must be specified.
func (s *Scheme) New(kind schema.GroupVersionKind) (Object, error) {
	if t, exists := s.gvkToType[kind]; exists {
		return reflect.New(t).Interface().(Object), nil
	}

	if t, exists := s.unversionedKinds[kind.Kind]; exists {
		return reflect.New(t).Interface().(Object), nil
	}
	return nil, NewNotRegisteredErrForKind(s.schemeName, kind)
}
```

- `scheme.IsGroupRegistered`：判断资源组是否已经注册

```go
// IsGroupRegistered returns true if types for the group have been registered with the scheme
func (s *Scheme) IsGroupRegistered(group string) bool {
	for _, observedVersion := range s.observedVersions {
		if observedVersion.Group == group {
			return true
		}
	}
	return false
}
```

- `scheme.IsVersionRegistered`：判断指定的 `GV` 是否注册

```go
// IsVersionRegistered returns true if types for the version have been registered with the scheme
func (s *Scheme) IsVersionRegistered(version schema.GroupVersion) bool {
	for _, observedVersion := range s.observedVersions {
		if observedVersion == version {
			return true
		}
	}

	return false
}
```

- `scheme.Recognizes`：判断指定的 `GVK` 是否已经注册

```go
// Recognizes returns true if the scheme is able to handle the provided group,version,kind
// of an object.
func (s *Scheme) Recognizes(gvk schema.GroupVersionKind) bool {
	_, exists := s.gvkToType[gvk]
	return exists
}
```

- `scheme.IsUnversioned`：判断指定的资源对象是否属于 `UnversionedType` 类型

```go
func (s *Scheme) IsUnversioned(obj Object) (bool, bool) {
	v, err := conversion.EnforcePtr(obj)
	if err != nil {
		return false, false
	}
	t := v.Type()

	if _, ok := s.typeToGVK[t]; !ok {
		return false, false
	}
	_, ok := s.unversionedTypes[t]
	return ok, true
}
```


