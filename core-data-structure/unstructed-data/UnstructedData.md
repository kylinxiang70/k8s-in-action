# Unstructed Data

## 结构化数据 Unstructed Data

数据可以分为结构化数据和非结构化数据。

- 结构化数据：预先知道结构的数据，例如 Json

```json
{
  "id": 1,
  "name": "kylinxiang"
}
```

- 处理结构化数据，需要一个对应的结构体

```go
package main

import (
	"encoding/json"
	"fmt"
)

// Student stores student info
type Student struct {
	ID   int
	Name string
}

func main() {
	s := `{"id": 1, "name": "kylinxiang"}`
	var student Student
	err := json.Unmarshal([]byte(s), &student)
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Printf("%v", student)
}
```

## 非结构化数据 Unstructed Data

非结构化数据：无法预知数据类型或属性名称，无法通过构建预定的 struct 数据结构来序列化和反序列化数据。例如：

```json
{
  "id": 1,
  "name": "kylinxiang",
  "description": ...  //无法预知其数据类型，所以不能处理
}
  
```

因为 Go 语言是强类型语言，它需要预先知道数据类型，所以在处理 JSON 数据时不如动态语言便捷。
当无法预知数据结构类型和属性名称时，可以使用如下的结构来解决问题：

```go
var result map[string]interface{}
```

每个字符串对应一个 JSON 属性，其映射 `interface{}` 类型对应值，可以是任意类型。
使用 `interface{}` 字段时，通过 Go 语言类型断言的方式进行类型转换。

```go
package main

import "fmt"

func main() {
	result := make(map[string]interface{})
	result["description"] = "kylinxiang"
	if description, ok := result["description"].(string); ok {
		fmt.Println(description)
	}
}

```

## Kubernetes非结构化数据处理

代码路径：vendor/k8s.io/apimachinery/pkg/runtime/interfaces.go

```go
// Unstructured objects store values as map[string]interface{}, with only values that can be serialized
// to JSON allowed.
type Unstructured interface {
	Object
	// NewEmptyInstance returns a new instance of the concrete type containing only kind/apiVersion and no other data.
	// This should be called instead of reflect.New() for unstructured types because the go type alone does not preserve kind/apiVersion info.
	NewEmptyInstance() Unstructured
	// UnstructuredContent returns a non-nil map with this object's contents. Values may be
	// []interface{}, map[string]interface{}, or any primitive type. Contents are typically serialized to
	// and from JSON. SetUnstructuredContent should be used to mutate the contents.
	UnstructuredContent() map[string]interface{}
	// SetUnstructuredContent updates the object content to match the provided map.
	SetUnstructuredContent(map[string]interface{})
	// IsList returns true if this type is a list or matches the list convention - has an array called "items".
	IsList() bool
	// EachListItem should pass a single item out of the list as an Object to the provided function. Any
	// error should terminate the iteration. If IsList() returns false, this method should return an error
	// instead of calling the provided function.
	EachListItem(func(Object) error) error
}

// Object interface must be supported by all API types registered with Scheme. Since objects in a scheme are
// expected to be serialized to the wire, the interface an Object must provide to the Scheme allows
// serializers to set the kind, version, and group the object is represented as. An Object may choose
// to return a no-op ObjectKindAccessor in cases where it is not expected to be serialized.
type Object interface {
	GetObjectKind() schema.ObjectKind
	DeepCopyObject() Object
}
```

代码路径：vendor/k8s.io/apimachinery/pkg/apis/meta/v1/unstructed/unstructed.go

```go
type Unstructured struct {
	// Object is a JSON compatible map with string, float, int, bool, []interface{}, or
	// map[string]interface{}
	// children.
	Object map[string]interface{}
}
```

上述代码中，Kubernetes非结构化数据通过 `map[string]interface{}` 表达，并提供接口。
在 client-go 编程式交互的 DynamicClient 内部，
实现了 `Unstructed` 类型，用于处理非结构化数据。
