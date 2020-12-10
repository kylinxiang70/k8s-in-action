# Codec编解码器

## 编解码器和序列化器的关系

- Serializer: 序列化器, 包含序列化和反序列化操作. 
- Codec: 编解码器, 包含编码器和解码器. 编解码器是一种术语, 指的是可以表示数据的任何格式, 
或者将数据转化成另外一种格式的过程. 所以, 可以将 Serializer 序列化器也理解成 Codec 编码器的一种. 



Codec 编解码器通用接口定义如下: 

```go
// Encoder writes objects to a serialized form
type Encoder interface {
	// Encode writes an object to a stream. Implementations may return errors if the versions are
	// incompatible, or if no conversion is defined.
	Encode(obj Object, w io.Writer) error
	// Identifier returns an identifier of the encoder.
	// Identifiers of two different encoders should be equal if and only if for every input
	// object it will be encoded to the same representation by both of them.
	//
	// Identifier is intended for use with CacheableObject#CacheEncode method. In order to
	// correctly handle CacheableObject, Encode() method should look similar to below, where
	// doEncode() is the encoding logic of implemented encoder:
	//   func (e *MyEncoder) Encode(obj Object, w io.Writer) error {
	//     if co, ok := obj.(CacheableObject); ok {
	//       return co.CacheEncode(e.Identifier(), e.doEncode, w)
	//     }
	//     return e.doEncode(obj, w)
	//   }
	Identifier() Identifier
}

// Decoder attempts to load an object from data.
type Decoder interface {
	// Decode attempts to deserialize the provided data using either the innate typing of the scheme or the
	// default kind, group, and version provided. It returns a decoded object as well as the kind, group, and
	// version from the serialized data, or an error. If into is non-nil, it will be used as the target type
	// and implementations may choose to use it rather than reallocating an object. However, the object is not
	// guaranteed to be populated. The returned object is not guaranteed to match into. If defaults are
	// provided, they are applied to the data by default. If no defaults or partial defaults are provided, the
	// type of the into may be used to guide conversion decisions.
	Decode(data []byte, defaults *schema.GroupVersionKind, into Object) (Object, *schema.GroupVersionKind, error)
}

// Serializer is the core interface for transforming objects into a serialized format and back.
// Implementations may choose to perform conversion of the object, but no assumptions should be made.
type Serializer interface {
	Encoder
	Decoder
}

// codec is a Serializer that deals with the details of versioning objects. It offers the same
// interface as Serializer, so this is a marker to consumers that care about the version of the objects
// they receive.
type Codec Serializer
```

从定义可以看出, 只要实现了 `Encoder` 和 `Decoder` 数据结构都是序列化器. 
每种序列化器都对资源对象的 `metav1.TypeMeta`（即 `APIVersion` 和 `Kind` 字段）进行验证, 
如果资源对象没有提供这些字段, 就会返回错误. 每种序列化器分别实现了 `Encode` 序列化方法和 `Decode` 反序列化方法: 

- `jsonSerializer`: Json 格式的序列化和反序列化器. `ContentType: application/json`
- `yamlSerializer`: Yaml 格式的序列化和反序列化器. `ContentType: application/yaml`
- `protobufSerializer`: Protobuf 格式序列化和反序列化器. `ContentType: application/vnd.kubernetes.protobuf`

三种 `ContentType` 定义在下面的文件中: 

代码路径: k8s.io/apimachinery/pkg/runtime/types.go

```go
const (
	ContentTypeJSON     string = "application/json"
	ContentTypeYAML     string = "application/yaml"
	ContentTypeProtobuf string = "application/vnd.kubernetes.protobuf"
)
```

### Codec编解码实例化

Codec 编解码器通过 NewCodeFctory 函数实例化, 实例化过程中会将 `jsonSerializer`、`yamlSerializer` 和 `protobufSerializer` 都实例化. 

NewCodeFctory --> newSerializerForScheme

```go
func newSerializersForScheme(scheme *runtime.Scheme, mf json.MetaFactory, options CodecFactoryOptions) []serializerType {
	jsonSerializer := json.NewSerializerWithOptions(
		mf, scheme, scheme,
		json.SerializerOptions{Yaml: false, Pretty: false, Strict: options.Strict},
	)
	jsonSerializerType := serializerType{
		AcceptContentTypes: []string{runtime.ContentTypeJSON},
		ContentType:        runtime.ContentTypeJSON,
		FileExtensions:     []string{"json"},
		EncodesAsText:      true,
		Serializer:         jsonSerializer,

		Framer:           json.Framer,
		StreamSerializer: jsonSerializer,
	}
	if options.Pretty {
		jsonSerializerType.PrettySerializer = json.NewSerializerWithOptions(
			mf, scheme, scheme,
			json.SerializerOptions{Yaml: false, Pretty: true, Strict: options.Strict},
		)
	}

	yamlSerializer := json.NewSerializerWithOptions(
		mf, scheme, scheme,
		json.SerializerOptions{Yaml: true, Pretty: false, Strict: options.Strict},
	)
	protoSerializer := protobuf.NewSerializer(scheme, scheme)
	protoRawSerializer := protobuf.NewRawSerializer(scheme, scheme)

	serializers := []serializerType{
		jsonSerializerType,
		{
			AcceptContentTypes: []string{runtime.ContentTypeYAML},
			ContentType:        runtime.ContentTypeYAML,
			FileExtensions:     []string{"yaml"},
			EncodesAsText:      true,
			Serializer:         yamlSerializer,
		},
		{
			AcceptContentTypes: []string{runtime.ContentTypeProtobuf},
			ContentType:        runtime.ContentTypeProtobuf,
			FileExtensions:     []string{"pb"},
			Serializer:         protoSerializer,

			Framer:           protobuf.LengthDelimitedFramer,
			StreamSerializer: protoRawSerializer,
		},
	}

	for _, fn := range serializerExtensions {
		if serializer, ok := fn(scheme); ok {
			serializers = append(serializers, serializer)
		}
	}
	return serializers
}
```

## jsonSerializer和yamlSerializer序列化器

`jsonSerializer` 使用Go语言标准库 `encoding/json` 来实现序列化和反序列化. 

`yamlSerializer` 使用第三方库 `gopkg.in/yaml.v2` 来实现序列化和反序列化. 

### 序列化操作

代码路径: vendor/k8s.io/apimachinery/pkg/runtime/serializer/json/json/go

`Encode` 函数支持两种序列化操作, 分别是 YAML 和 JOSN. 

```go
// Encode serializes the provided object to the given writer.
func (s *Serializer) Encode(obj runtime.Object, w io.Writer) error {
	if co, ok := obj.(runtime.CacheableObject); ok {
		return co.CacheEncode(s.Identifier(), s.doEncode, w)
	}
	return s.doEncode(obj, w)
}

func (s *Serializer) doEncode(obj runtime.Object, w io.Writer) error {
	if s.options.Yaml { //如果是YAML格式
		json, err := caseSensitiveJSONIterator.Marshal(obj) // 先将对象转化为JSON格式
		if err != nil {
			return err
		}
		data, err := yaml.JSONToYAML(json) //在将JSON格式转化为YAML格式
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
    // 如果是JSON格式
	if s.options.Pretty { //如果开启了Pretty
		data, err := caseSensitiveJSONIterator.MarshalIndent(obj, "", "  ") // 优化格式
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	encoder := json.NewEncoder(w) // 通过Go语言标准库的json.Encode函数将资源对象转化为JSON格式. 
	return encoder.Encode(obj)
}
```

Kubernetes 在 `jsonSerializer` 序列化器上做了一些优化, 
`caseSensitiveJsonIterator` 函数实际封装了 `github.com/json-iterator/go` 第三方库, 
`json-iterator` 有以下几个好处: 

- json-iterator 支持区分大小写, encoding/json 不支持. 
- json-iterator 性能更优, 编码可达到 837ns/op, 解码可达到 5623ns/op. 
- json-iterator 100%兼容 Go 语言标准库 encoding/json, 可随时切换两种编码方式. 



### 反序列化操作

代码路径: vendor/k8s.io/apimachinery/pkg/runtime/serializer/json/json/go

同样 Decode 函数也支持两种序列化操作, 分别是 YAML 和 JOSN

```go
// Decode attempts to convert the provided data into YAML or JSON, extract the stored schema kind, apply the provided default gvk, and then
// load that data into an object matching the desired schema kind or the provided into.
// If into is *runtime.Unknown, the raw data will be extracted and no decoding will be performed.
// If into is not registered with the typer, then the object will be straight decoded using normal JSON/YAML unmarshalling.
// If into is provided and the original data is not fully qualified with kind/version/group, the type of the into will be used to alter the returned gvk.
// If into is nil or data's gvk different from into's gvk, it will generate a new Object with ObjectCreater.New(gvk)
// On success or most errors, the method will return the calculated schema kind.
// The gvk calculate priority will be originalData > default gvk > into
func (s *Serializer) Decode(originalData []byte, gvk *schema.GroupVersionKind, into runtime.Object) (runtime.Object, *schema.GroupVersionKind, error) {
	data := originalData
	if s.options.Yaml {  // 如果是YAML格式的数据
		altered, err := yaml.YAMLToJSON(data)   // 先转化成JSON格式-
		if err != nil {
			return nil, nil, err
		}
		data = altered  // 之后处理的格式都是JSON
	}

	actual, err := s.meta.Interpret(data)  // actual 是JSON解析出来的GVK类型
	if err != nil {
		return nil, nil, err
	}

	if gvk != nil {
		*actual = gvkWithDefaults(*actual, *gvk) // 如果actual version和kind等字段为空, 则使用默认值
	}

	if unk, ok := into.(*runtime.Unknown); ok && unk != nil { // 如果into是unknown类型, 则返回unknown类型
		unk.Raw = originalData
		unk.ContentType = runtime.ContentTypeJSON
		unk.GetObjectKind().SetGroupVersionKind(*actual)
		return unk, actual, nil
	}

	if into != nil {
		_, isUnstructured := into.(runtime.Unstructured)
		types, _, err := s.typer.ObjectKinds(into)
		switch {
		case runtime.IsNotRegisteredError(err), isUnstructured:
			if err := caseSensitiveJSONIterator.Unmarshal(data, into); err != nil {
				return nil, actual, err
			}
			return into, actual, nil
		case err != nil:
			return nil, actual, err
		default:
			*actual = gvkWithDefaults(*actual, types[0])
		}
	}

	if len(actual.Kind) == 0 {
		return nil, actual, runtime.NewMissingKindErr(string(originalData))
	}
	if len(actual.Version) == 0 {
		return nil, actual, runtime.NewMissingVersionErr(string(originalData))
	}

	// use the target if necessary
	obj, err := runtime.UseOrCreateObject(s.typer, s.creater, *actual, into)
	if err != nil {
		return nil, actual, err
	}

	if err := caseSensitiveJSONIterator.Unmarshal(data, obj); err != nil {
		return nil, actual, err
	}

	// If the deserializer is non-strict, return successfully here.
	if !s.options.Strict {
		return obj, actual, nil
	}

	// In strict mode pass the data trough the YAMLToJSONStrict converter.
	// This is done to catch duplicate fields regardless of encoding (JSON or YAML). For JSON data,
	// the output would equal the input, unless there is a parsing error such as duplicate fields.
	// As we know this was successful in the non-strict case, the only error that may be returned here
	// is because of the newly-added strictness. hence we know we can return the typed strictDecoderError
	// the actual error is that the object contains duplicate fields.
	altered, err := yaml.YAMLToJSONStrict(originalData)
	if err != nil {
		return nil, actual, runtime.NewStrictDecodingError(err.Error(), string(originalData))
	}
	// As performance is not an issue for now for the strict deserializer (one has regardless to do
	// the unmarshal twice), we take the sanitized, altered data that is guaranteed to have no duplicated
	// fields, and unmarshal this into a copy of the already-populated obj. Any error that occurs here is
	// due to that a matching field doesn't exist in the object. hence we can return a typed strictDecoderError,
	// the actual error is that the object contains unknown field.
	strictObj := obj.DeepCopyObject()
	if err := strictCaseSensitiveJSONIterator.Unmarshal(altered, strictObj); err != nil {
		return nil, actual, runtime.NewStrictDecodingError(err.Error(), string(originalData))
	}
	// Always return the same object as the non-strict serializer to avoid any deviations.
	return obj, actual, nil
}
```
