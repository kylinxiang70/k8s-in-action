# Convert资源版本转换器

# Convert资源版本转换器

Kubernetes 允许同一资源的不同版本进行准换, 进行转换之前, 需要查看资源对象支持的资源组和资源版本:
 
```json
$ kubectl version
Client Version: version.Info{
  Major:"1", 
  Minor:"16+", 
  GitVersion:"v1.16.6-beta.0", 
  GitCommit:"e7f962ba86f4ce7033828210ca3556393c377bcc", 
  GitTreeState:"clean", 
  BuildDate:"2020-01-15T08:26:26Z", 
  GoVersion:"go1.13.5", 
  Compiler:"gc", 
  Platform:"darwin/amd64"
}
Server Version: version.Info{
  Major:"1", 
  Minor:"19", 
  GitVersion:"v1.19.2", 
  GitCommit:"f5743093fd1c663cb0cbc89748f730662345d44d", 
  GitTreeState:"clean", 
  BuildDate:"2020-09-16T13:32:58Z", 
  GoVersion:"go1.15", 
  Compiler:"gc", 
  Platform:"linux/amd64"
}

$ kubectl api-versions | grep autoscaling
autoscaling/v1
autoscaling/v2beta1
autoscaling/v2beta2

$ kubectl api-resources --api-group=autoscaling
NAME                       SHORTNAMES   APIGROUP      NAMESPACED   KIND
horizontalpodautoscalers   hpa          autoscaling   true         HorizontalPodAutoscaler
```

通过 kubectl version、kubectl api-versions、kubectl api-resources 
查看当前版本 Kubernetes 集群中的 autoscaling 资源组下的资源版本, 以及 Kind. 
可以发现 v1.19.2 集群中的 autoscaling 资源组有三个资源版本, 分别为 v1/v2beta1/v2beta2. 
并且 autoscaling 资源组下拥有 `HorizontalPodAutoscaler` 资源对象. 

比如将对象 `HorizontalPodAutoscaler` 版本的 v1 转换为 v2beta1: 

```yaml
# v1版本的HPA
apiVersion: autoscaling/v1
kind: HorizontalPodAutoscaler
metadata:
  labels:
    hpa: my-temp-hpa
  name: my-temp-hpa
spec:
  maxReplicas: 10
  minReplicas: 1
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-temp
  targetCPUUtilizationPercentage: 20
```

通过以下命令将其转换为 v2beta1 版本

```
$ kubectl convert -f hpa_v1_v1beta1.yaml --output-version=autoscaling/v2beta1
kubectl convert is DEPRECATED and will be removed in a future version.
In order to convert, kubectl apply the object to the cluster, then kubectl get at the desired version.
apiVersion: autoscaling/v2beta1
kind: HorizontalPodAutoscaler
metadata:
  creationTimestamp: null
  labels:
    hpa: my-temp-hpa
  name: my-temp-hpa
spec:
  maxReplicas: 10
  metrics:
  - resource:
      name: cpu
      targetAverageUtilization: 20
    type: Resource
  minReplicas: 1
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-temp
status:
  conditions: null
  currentMetrics: null
  currentReplicas: 0
  desiredReplicas: 0
```

---

为了解决多个版本之间的转换问题, Kubernetes 通过内部版本 `__internal` 进行多个外部版本之间的转换. 

## Convert转换器数据结构

Convert 转换器数据结构主要存放转换函数（即 Conversion Funs）. Convert 数据结构如下: 

代码路径: vendor/k8s.io/apimachinery/pkg/conversion/convert.go

```go
// Converter knows how to convert one type to another.
type Converter struct {
	// Map from the conversion pair to a function which can
	// do the conversion.
	conversionFuncs          ConversionFuncs // 默认的转换函数, 一般定义在资源目录下conversion.go中
	generatedConversionFuncs ConversionFuncs // 自动生成的转换函数, 一般定义在资源目录下zz_generated.conversion.go目录中

	// Set of conversions that should be treated as a no-op
	ignoredConversions        map[typePair]struct{} // 若资源对象注册到此字段, 则忽略此资源对象的转换操作
	ignoredUntypedConversions map[typePair]struct{}

	// nameFunc is called to retrieve the name of a type; this name is used for the
	// purpose of deciding whether two types match or not (i.e., will we attempt to
	// do a conversion). The default returns the go type name.
    // // 在转换过程中获取资源种类的名称, 该函数被定义在vendor/k8s.io/apimachinery/pkg/runtime/scheme.go
	nameFunc func(t reflect.Type) string 
}

```

Convert 转换器数据结构中存放的转换函数（即 Conversion Funcs）可以分为两类, 
分别为默认的转换函数（即 ConversionFuncs）字段和自动生成的转换函数（即 generatedConversionFuncs）字段. 
它们都通过 ConversionFuncs 来管理, 代码示例如下: 

```go
type ConversionFuncs struct {
	untyped map[typePair]ConversionFunc
}

type typePair struct {
	source reflect.Type
	dest   reflect.Type
}

// ConversionFunc converts the object a into the object b, reusing arrays or objects
// or pointers if necessary. It should return an error if the object cannot be converted
// or if some data is invalid. If you do not wish a and b to share fields or nested
// objects, you must copy a before calling this function.
type ConversionFunc func(a, b interface{}, scope Scope) error

// scope contains information about an ongoing conversion.
type scope struct {
	converter *Converter
	meta      *Meta
}
```

`ConversionFunc` 类型函数（即 Type Function）定义了转换函数实现的结构, 
将资源 a 转换为资源 b. a 定义了源资源类型, b 定义了目标资源类型. 
scope 定义了多次转换机制（即递归调用转换函数）. 

## Converter注册转换函数

Converter 转换函数需要注册才能在 Kubernetes 内部使用, Kubernetes 支持一下注册函数: 

代码路径: vendor/k8s.io/apimachinery/pkg/runtime/scheme.go

```go
// AddIgnoredConversionType identifies a pair of types that should be skipped by
// conversion (because the data inside them is explicitly dropped during
// conversion).
func (s *Scheme) AddIgnoredConversionType(from, to interface{}) error {
	return s.converter.RegisterIgnoredConversion(from, to)
}

// AddConversionFunc registers a function that converts between a and b by passing objects of those
// types to the provided function. The function *must* accept objects of a and b - this machinery will not enforce
// any other guarantee.
func (s *Scheme) AddConversionFunc(a, b interface{}, fn conversion.ConversionFunc) error {
	return s.converter.RegisterUntypedConversionFunc(a, b, fn)
}

// AddGeneratedConversionFunc registers a function that converts between a and b by passing objects of those
// types to the provided function. The function *must* accept objects of a and b - this machinery will not enforce
// any other guarantee.
func (s *Scheme) AddGeneratedConversionFunc(a, b interface{}, fn conversion.ConversionFunc) error {
	return s.converter.RegisterGeneratedUntypedConversionFunc(a, b, fn)
}

// AddFieldLabelConversionFunc adds a conversion function to convert field selectors
// of the given kind from the given version to internal version representation.
func (s *Scheme) AddFieldLabelConversionFunc(gvk schema.GroupVersionKind, conversionFunc FieldLabelConversionFunc) error {
	s.fieldLabelConversionFuncs[gvk] = conversionFunc
	return nil
}
```

- `AddIgnoredConversionType`: 注册忽略的资源类型, 不会执行转换操作
- `AddConversionFunc`: 注册单个 `ConvertionFunc` 转换函数
- `AddGeneratedConversionFunc`: 注册自动生成的转换函数
- `AddFieldLabelConversionFunc`: 注册字段标签（`FieldLabel`）的转换函数
