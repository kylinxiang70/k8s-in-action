# Informer

## 简介

在 kubernetes 系统中, 组件之间通过 http 协议进行通信, 
通过 informer 来做到了消息的实时性、可靠性、顺序性, 
通过 informer 机制与 api-server 进行通信.
Informer 的机制, 降低了了 Kubernetes 各个组件跟 Etcd 与 Kubernetes API Server 的通信压力.
## 架构设计

![informer架构](img/informer-arch.jpg)

这张图分为两部分, 黄色图标是开发者需要自行开发的部分, 而其它的部分是 client-go 已经提供的, 直接使用即可.

Informer架构设计中有多个核心组件: 

1. **Reflector**: 
用于 Watch 指定的 Kubernetes 资源, 当 watch 的资源发生变化时, 触发变更的事件, 
比如 Added, Updated 和 Deleted 事件, 并将资源对象存放到本地缓存 DeltaFIFO; 

2. **DeltaFIFO**: 
拆开理解, FIFO 就是一个队列, 拥有队列基本方法(ADD, UPDATE, DELETE, LIST, POP, CLOSE 等), 
Delta 是一个资源对象存储, 保存存储对象的消费类型, 比如 Added, Updated, Deleted, Sync 等; 

3. **Indexer**: Client-go 用来存储资源对象并自带索引功能的本地存储, 
Reflector 从 DeltaFIFO 中将消费出来的资源对象存储到 Indexer, 
Indexer 与 Etcd 集群中的数据完全保持一致.
从而 client-go 可以本地读取, 减少 Kubernetes API 和 Etcd 集群的压力.

### Informer 使用示例

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

### Informer 实现

1. 资源 Informer

每一个 Kubernetes 资源都实现了 Informer 机制. 
每一个 Informer 都会实现 Informer 和 List 方法, 例如 PodInformer, 代码示例如下:

`vendor/k8s.io/client-go/informers/core/v1/pod.go`

```go
// PodInformer provides access to a shared informer and lister for
// Pods.
type PodInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.PodLister
}
```

调用不同资源的 Informer, 代码示例如下: 

```go
podInformer := sharedInformers.Core().V1().Pods().Informer()
nodeInformer := sharedInformers.Node().V1beta1().RuntimeClasses().Informer()
```

2. SharedInformer 共享机制

Informer 也被成为 SharedInformer,可以被共享使用.
在使用 Client-go 编写程序时, 若同一个资源的 Informer 被实例化了多次, 每个 Informer 都使用一个 Reflector, 
那么会运行多个相同的 ListAndWatch, 太多重复的序列化和反序列化会导致 Kubernetes API Server 负载过重.

Shared Informer 可以使用同一类资源 Informer 共享一个 Reflector, 这样可以节约很多资源. 
通过 map 数据结构实现共享 Informer 的机制. Shared Informer定义了一个 map 数据结构, 
用于存放所有 Informer 的字段,代码示例如下: 

```go
type sharedInformerFactory struct {
    ...
	informers map[reflect.Type]cache.SharedIndexInformer
}

// InternalInformerFor returns the SharedIndexInformer for obj using an internal
// client.
func (f *sharedInformerFactory) InformerFor(obj runtime.Object, newFunc internalinterfaces.NewInformerFunc) cache.SharedIndexInformer {
	f.lock.Lock()
	defer f.lock.Unlock()

	informerType := reflect.TypeOf(obj)
	informer, exists := f.informers[informerType]
	if exists {
		return informer
	}

	resyncPeriod, exists := f.customResync[informerType]
	if !exists {
		resyncPeriod = f.defaultResync
	}

	informer = newFunc(f.client, resyncPeriod)
	f.informers[informerType] = informer

	return informer
}
```

informers 字段中存储了资源类型和对应于 SharedIndexInformer 的映射关系.
Informer 函数添加了不同资源的 Informer, 在添加过程中如果已经存在同类型的资源 Informer,
则返回当前 Informer, 不再继续添加.

最后通过 Shared Informer 的 Start 方法使 f.informers 中的每一个 informer 通过 goroutine 持久运行.

```go
// Start initializes all requested informers.
func (f *sharedInformerFactory) Start(stopCh <-chan struct{}) {
	f.lock.Lock()
	defer f.lock.Unlock()

	for informerType, informer := range f.informers {
		if !f.startedInformers[informerType] {
			go informer.Run(stopCh)
			f.startedInformers[informerType] = true
		}
	}
}
```