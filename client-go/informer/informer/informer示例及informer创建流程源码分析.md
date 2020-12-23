#  Informer 示例及源码分析

## Informer 使用示例

```go
package main

import (
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"log"
	"path"
	"time"
)

func main() {
	homedir := homedir.HomeDir()
	configPath := path.Join(homedir, ".kube/config")
	fmt.Println(configPath)
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		panic(err)
	}

	// 创建 ClientSet 对象, Informer 需要通过 ClientSet 和 Kubernetes API server 通信
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// 创建 stopCh 对象, 该对象用于在应用程序退出之前通知 Informer 提前退出
	stopCh := make(chan struct{})
	defer close(stopCh)

	// NewSharedInformerFactory(client kubernetes.Interface, defaultResync time.Duration) SharedInformerFactory
	// 实例化 sharedInformer 对象, 它接收两个参数:
	// 1. client: 用于和 Kubernetes API server通信
	// 2. defaultResync: 设置多久时间进行一次 Resync (重新同步), Resync会周期性执行List操作, 
	//       将所有资源都存放在 Informer Store 中, 如果该参数为0, 则不同步.
	sharedInformers := informers.NewSharedInformerFactory(clientSet, time.Minute)

	// 得到具体资源对象的informer对象
	informer := sharedInformers.Core().V1().Pods().Informer()

	// 通过 informer.AddEventHandler函数为资源对象添加资源对象的回调方法, 支持3种资源事件的回调方法:
	// 1. AddFunc: 当创建资源时触发的事件回调方法
	// 2. UpdateFunc: 当更新资源时触发的事件回调方法
	// 3. DeleteFunc: 当删除资源时触发的事件回调用法
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// Kubernetes 使用 Informer机制时触发资源的回调方法, 将资源对象推送到 WorkQueue 或其他队列, 
		// 这里直接触发资源的事件.
		AddFunc: func(obj interface{}) {
			mObj := obj.(v1.Object)
			log.Printf("New Pod Added to Store: %s", mObj.GetName())
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oObj := oldObj.(v1.Object)
			nObj := newObj.(v1.Object)
			log.Printf("%s with UID %s has been updated to %s with %s",
				nObj.GetName(), nObj.GetUID(), oObj.GetName(), oObj.GetUID())
		},
		DeleteFunc: func(obj interface{}) {
			dObje := obj.(v1.Object)
			log.Printf("Resource %s has been deleted.", dObje.GetName())
		},
	})

	// 运行 informer
	informer.Run(stopCh)
}
```

## Informer 创建过程详解

本创建过程源码分析基于上述 informer 示例

### sharedInformerFactor 创建过程详解

这里以上面实例中的 sharedInformerFactor 为例, 使用  NewSharedInformerFactory 创建 sharedInformerFactor.

```go
// 示例
sharedInformers := informers.NewSharedInformerFactory(clientSet, time.Minute)
```

1. NewSharedInformerFactory

```go
// 通过 NewSharedInformerFactory 创建 sharedInformerFactor, 
// client 参数为与 k8s 交互的客户端, 只要实现了 kubernetes.Interface 接口.
// defaultResync 为Informer 与客户端同步的时间间隔.
// NewSharedInformerFactory 实际调用了 NewSharedInformerFactoryWithOptions 实现.
func NewSharedInformerFactory(client kubernetes.Interface, defaultResync time.Duration) SharedInformerFactory {
	return NewSharedInformerFactoryWithOptions(client, defaultResync)
}
```

2. NewSharedInformerFactoryWithOptions

```go
// NewSharedInformerFactoryWithOptions 相对于 NewSharedInformerFactory 多了 options 选项用于创建特定选项的
// SharedInformerFactory, 最后返回一个 sharedInformerFactory 对象的指针.
func NewSharedInformerFactoryWithOptions(client kubernetes.Interface, defaultResync time.Duration, options ...SharedInformerOption) SharedInformerFactory {
	factory := &sharedInformerFactory{
		client:           client,
		namespace:        v1.NamespaceAll,
		defaultResync:    defaultResync,
		informers:        make(map[reflect.Type]cache.SharedIndexInformer),
		startedInformers: make(map[reflect.Type]bool),
		customResync:     make(map[reflect.Type]time.Duration),
	}

	// Apply all options
	for _, opt := range options {
		factory = opt(factory)
	}

	return factory
}
```

3. sharedInformerFactory

```go
type sharedInformerFactory struct {
  // 调用 NewSharedInformerFactory 传入的 client 类型
	client           kubernetes.Interface
  // 指定命名空间
	namespace        string
  // 
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	lock             sync.Mutex
  // 调用 NewSharedInformerFactory 传入的同步时间
	defaultResync    time.Duration
  // 存储了每种资源的同步间隔时间
	customResync     map[reflect.Type]time.Duration
  // 存储了资源类型和对应于 SharedIndexInformer 的映射关系
	informers map[reflect.Type]cache.SharedIndexInformer
  // 用来记录被启动的 informer. 以便于 Start() 函数被多次安全调用
	startedInformers map[reflect.Type]bool
}
```

### 使用 sharedInformerFactor 创建 SharedIndexInformer

这里使用 podInformer 介绍 sharedInformerFactor 创建的过程.

```go
informer := sharedInformers.Core().V1().Pods().Informer()
```

1. Core()

```go
// Core() 实际调用 core.group 结构体的 core.New() 方法构造了一个新的 group 对象
func (f *sharedInformerFactory) Core() core.Interface {
	return core.New(f, f.namespace, f.tweakListOptions)
}
```

1.1.  core.New()

```go
// New returns a new Interfa
// New() 返回了一个 group 对象指针, 该对象用来存储资源组的相关信息
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &group{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}
```

2. V1()

```go
// V1 returns a new v1.Interface.
// V1() 调用了 v1 结构体的 New() 方法创建了一个新的 v1.Interface
func (g *group) V1() v1.Interface {
	return v1.New(g.factory, g.namespace, g.tweakListOptions)
}

// v1.Interface 提供了所有 core 资源组下的 informer 访问方式
type Interface interface {
	// ComponentStatuses returns a ComponentStatusInformer.
	ComponentStatuses() ComponentStatusInformer
	// ConfigMaps returns a ConfigMapInformer.
	ConfigMaps() ConfigMapInformer
	// Endpoints returns a EndpointsInformer.
	Endpoints() EndpointsInformer
	// Events returns a EventInformer.
	Events() EventInformer
	// LimitRanges returns a LimitRangeInformer.
	LimitRanges() LimitRangeInformer
	// Namespaces returns a NamespaceInformer.
	Namespaces() NamespaceInformer
	// Nodes returns a NodeInformer.
	Nodes() NodeInformer
	// PersistentVolumes returns a PersistentVolumeInformer.
	PersistentVolumes() PersistentVolumeInformer
	// PersistentVolumeClaims returns a PersistentVolumeClaimInformer.
	PersistentVolumeClaims() PersistentVolumeClaimInformer
	// Pods returns a PodInformer.
	Pods() PodInformer
	// PodTemplates returns a PodTemplateInformer.
	PodTemplates() PodTemplateInformer
	// ReplicationControllers returns a ReplicationControllerInformer.
	ReplicationControllers() ReplicationControllerInformer
	// ResourceQuotas returns a ResourceQuotaInformer.
	ResourceQuotas() ResourceQuotaInformer
	// Secrets returns a SecretInformer.
	Secrets() SecretInformer
	// Services returns a ServiceInformer.
	Services() ServiceInformer
	// ServiceAccounts returns a ServiceAccountInformer.
	ServiceAccounts() ServiceAccountInformer
}
```

2.1 v1.New()

```go
// New returns a new Interface.
// New 创建了一个新的 version 对象.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// version 结构体
// version 实现了 v1.Interface 接口, 因此 version 可以直接访问该资源组下的资源对应的所有 informer；
type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}
```

3. Pods()

```go
// Pods returns a PodInformer.
// Pods() 返回了一个实现了 PodInformer 接口的 podInformer 对象
func (v *version) Pods() PodInformer {
	return &podInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// PodInformer provides access to a shared informer and lister for
// Pods.
// 每一个 Kubernetes 资源都实现了 Informer 机制. 
每一个 Informer 都会实现 Informer 和 List 方法
type PodInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.PodLister
}

// podInformer 结构, podInformer 实现了 PodInformer接口
type podInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// podInformer 分别实现了 Informer() 和 Lister()
func (f *podInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&corev1.Pod{}, f.defaultInformer)
}

func (f *podInformer) Lister() v1.PodLister {
	return v1.NewPodLister(f.Informer().GetIndexer())

```

4. Informer()

```go
// Informer() 调用 factory.InformerFor 方法返回 SharedIndexInformer.
func (f *podInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&corev1.Pod{}, f.defaultInformer)
}
```

4.1. ==InformerFor()== 

Informer 共享机制 ！！！

```go
// InternalInformerFor returns the SharedIndexInformer for obj using an internal
// client.
// Informer 也被称为 SharedInformer,可以被共享使用.
// 在使用 Client-go 编写程序时, 若同一个资源的 Informer 被实例化了多次, 每个 Informer 都使用一个 Reflector, 
// 那么会运行多个相同的 ListAndWatch, 太多重复的序列化和反序列化会导致 Kubernetes API Server 负载过重.
//
// Shared Informer 可以使用同一类资源 Informer 共享一个 Reflector, 这样可以节约很多资源. 
// 通过 map 数据结构实现共享 Informer 的机制. Shared Informer定义了一个 map 数据结构, 
// 用于存放所有 Informer 的字段
func (f *sharedInformerFactory) InformerFor(obj runtime.Object, newFunc internalinterfaces.NewInformerFunc) cache.SharedIndexInformer {
  // 保证并发安全, 因为可能多个协程同时获取 SharedIndexInformer
	f.lock.Lock()
	defer f.lock.Unlock()

  // 获取 reflect.Type 类型作为键
	informerType := reflect.TypeOf(obj)
  // 从 sharedInformerFactor 的 informers map中获取当前资源类型的 informer.
  // sharedInformerFactor 可以复习一下上面创建 sharedInformerFactor 的讲解.
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

  // 更新资源的同步时间
	resyncPeriod, exists := f.customResync[informerType]
	if !exists {
		resyncPeriod = f.defaultResync
	}

  // 使用 newFunc() 创建新的 informer.
  // newFunc() 为一个函数类型,实际为上一步传入的 f.defaultInformer() 方法
	informer = newFunc(f.client, resyncPeriod)
  // 将创建的 informer 加入 map.
	f.informers[informerType] = informer

	return informer
}
```

4.2 defaultInformer()

```go
// defaultInformer 使用 NewFilteredPodInformer 创建了 pod 对应的 informer.
func (f *podInformer) defaultInformer(client kubernetes.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredPodInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}
```

4.3 NewFilteredPodInformer()

```go
// NewFilteredPodInformer 创建了一个新的 pod 类型的 informer.
// 最好使用 informer factory 来创建一个共享的 informer, 而不是一个单独的 informer.
// 这有利于减少内存消耗和服务端的连接数量.
func NewFilteredPodInformer(client kubernetes.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
    // 创建 ListWatch 对象, 使用传入的 clientSet 中实现的针对 Pod 的 List 和 Watch 方法.
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().Pods(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.CoreV1().Pods(namespace).Watch(context.TODO(), options)
			},
		},
		&corev1.Pod{},
		resyncPeriod,
		indexers,
	)
}

// ListerWatcher 是任何知道如何执行初始列表并在资源上启 watch 的对象
type ListerWatcher interface {
	Lister
	Watcher
}

// Lister 是任何知道如何执行一个初始列表的对象
type Lister interface {
  // List 方法返回了一个列表类型的对象, Items 字段将被提取,
  // ResourceVersion 字段将会用来从正确的地方开始 watch
	List(options metav1.ListOptions) (runtime.Object, error)
}

// Watcher 是任何知道如何监控一个资源的对象.
type Watcher interface {
	// Watch should begin a watch at the specified version.
  // Watch 应该从指定的 version 开始 watch
	Watch(options metav1.ListOptions) (watch.Interface, error)
}

// listWatch 知道怎么 list 和 watch 一组 apiserver 的资源.
// 实现了 ListerWatcher 接口
// ListWatch 的函数可以被 NewReflect 等使用
// ListFunc 和 WatchFunc 不能为空
type ListWatch struct {
	ListFunc  ListFunc
	WatchFunc WatchFunc
	// DisableChunking requests no chunking for this list watcher.
	DisableChunking bool
}
```

4.4 NewSharedIndexInformer

```go
// NewSharedIndexInformer 创建了一个 listwather 的实例.
// 如果 defaultEventHandlerResyncPeriod 参数为0, 则创建的 informer 就不会更新.
// 否则, 每一个请求重新同步周期不为 0 的 handler, 不管该 handler 是在 informer 启动
// 之前或者之后添加的, 名义上的同步周期是请求同步时间间隔加上多个 informer 重新同步检测周期.
// 当 informer 被启动, informer 的同步检测周期随之被建立.
// informer 同步检测周期为 a 和 b 的较大值
// a: informer 启动前请求的同步周期和当前 defaultEventHandlerResyncPeriod 的较小值
// b: minimumResyncPeriod 的值 (1 * time.Second)
func NewSharedIndexInformer(lw ListerWatcher, exampleObject runtime.Object, defaultEventHandlerResyncPeriod time.Duration, indexers Indexers) SharedIndexInformer {
	realClock := &clock.RealClock{}
	sharedIndexInformer := &sharedIndexInformer{
		processor:                       &sharedProcessor{clock: realClock},
		indexer:                         NewIndexer(DeletionHandlingMetaNamespaceKeyFunc, indexers),
		listerWatcher:                   lw,
		objectType:                      exampleObject,
		resyncCheckPeriod:               defaultEventHandlerResyncPeriod,
		defaultEventHandlerResyncPeriod: defaultEventHandlerResyncPeriod,
		cacheMutationDetector:           NewCacheMutationDetector(fmt.Sprintf("%T", exampleObject)),
		clock:                           realClock,
	}
	return sharedIndexInformer
}
```

