# Kubernetes Informer 机制详解

## Kubernetes为什么需要Informer

Kubernetes是以声明式API为基础的容器编排系统，声明式API定义了很多状态，使得集群可以从某个状态向期望的状态趋近（最终一致性）。

Kubernetes是一个典型的master-slave架构，slave需要同步master的状态，即Node节点需要通过向API Server发送REST请求同步存储在etcd中的状态。在大型生产环境中，可能存在成千上万的slave节点，所以需要一个高效的、可靠的数据同步机制，因此有了Informer。

## Informer概述

在 kubernetes 中, Client（比如Kubelet）和APIServer之间通过 informer 来做到了消息的**<u>实时性、可靠性、顺序性</u>**，同时有效减轻了Kubernetes的负载。

### Informer架构设计

![Informer架构](D:\go\src\github\kylinxiang70\k8s-in-action\client-go\informer\img\informer-brief-arch.png)

Informer架构设计中有多个核心组件: 

1. **Reflector**: 使用`ListAndWatch`同步指定类型的 Kubernetes 资源, List用于全量同步指定的资源，Watch用于监控资源的变化。
   比如 Added, Updated 和 Deleted 事件, 并将资源对象存放到本地缓存 DeltaFIFO; 
2. **DeltaFIFO**: 拆开理解, FIFO 就是一个队列, 拥有队列基本方法(ADD, UPDATE, DELETE, LIST, POP, CLOSE 等), 
   Delta 是一个资源对象存储, 存储对应的事件修改事件类型和资源对象; 当DeltaFIFO的POP()方法被调用时，会触发Controller注册的回调函数来处理对应类型的事件。
3. **Indexer**: Client-go 用来存储资源对象并自带索引功能的本地存储, List和Watch到的资源都会存储在Indexer中, 
   Indexer 与 Etcd 的数据具有最终一致性关系.从而 client-go 可以本地读取, 减少 Kubernetes API 和 Etcd 集群的压力.
   - ThreadSafeStore
   - Indices

### 各组件之间的关系

## Demo

```go
/**
 * @author xiangqilin
 * @date 2020/12/8
**/
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

	// 创建 stopCh 对象，该对象用于在应用程序退出之前通知 Informer 退出
	stopCh := make(chan struct{})
	defer close(stopCh)

	// NewSharedInformerFactory(client kubernetes.Interface, defaultResync time.Duration) SharedInformerFactory
	// 实例化 sharedInformer 对象, 它接收两个参数:
	// 1. client: 用于和 Kubernetes API server通信
	// 2. defaultResync: 设置多久时间进行一次 Resync (重新同步), Resync会周期性执行List操作，
	//       将所有资源都存放在 Informer Store 中, 如果该参数为0, 则不同步.
	sharedInformers := informers.NewSharedInformerFactory(clientSet, time.Minute)

	// 得到具体资源对象的informer对象
	informer := sharedInformers.Core().V1().Pods().Informer()

	// 通过 informer.AddEventHandler函数为资源对象添加资源对象的回调方法，支持3种资源事件的回调方法:
	// 1. AddFunc: 当创建资源时触发的事件回调方法
	// 2. UpdateFunc: 当更新资源时触发的事件回调方法
	// 3. DeleteFunc: 当删除资源时触发的事件回调用法
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// Kubernetes 使用 Informer机制时触发资源的回调方法，将资源对象推送到 WorkQueue 或其他队列，
		// 这里直接触发资源的事件。
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

## Reflector

Reflector中包含一个ListerWacher对象，其实现了对特定资源的List和Watch。Refector首先会使用List请求全量同步数据，然后获取资源的ResourceVersion，基于ResourceVersion来Watch指定资源的变化情况。

### Reflector

```go
// Reflector watch 指定的资源，然后将所有的变化存储到 store.
type Reflector struct {
    // name 用来识别当前的 reflector. 默认是 file:line
	name string
    // 期望被放入 store 的类型名称。
    // 如果 expectedGVK 不为空，则 expectedTypeName 为 expectedGVK 的字符串形式；
    // 否则， expectedTypeName 的值为 expectedType 的字符串形式。
    // expectedTypeName 只起展示作用，并不用作解析和比较。
	expectedTypeName string
    // 期望被放入 store 的一个样例对象的类型。
    // 只需要类型是正确的，除非是 `unstructured.Unstructured`,
    // 但是 apiVersion 和 kind 也需要是正确的。
	expectedType reflect.Type
    // 期望被放入 store 的对象的 GVK
	expectedGVK *schema.GroupVersionKind
    // 与 watch 资源同步的目的地，DeltaFIFO 实现了store 接口
	store Store
    // listerWatcher 用来执行 lists 和 watches 操作。
	listerWatcher ListerWatcher
    // backoffManager 用来管理 ListWatch 的执行。
	backoffManager wait.BackoffManager
    // 重新同步的周期
	resyncPeriod time.Duration
    // ShouldResync 被周期性地调用，无论何时其返回 true，store 的 Resync 操作都会被调用。
	ShouldResync func() bool
    // clock 允许测试修改时间。
	clock clock.Clock
    // paginatedResult 定义是否强制 list 调用时进行分页。
    // 在初始 list 调用时被设置。
	paginatedResult bool
    // 上一次与依赖的 store 同步时观察到的 resource version token.
	lastSyncResourceVersion string
    // isLastSyncResourceVersionUnavailable 为 true，如果之前带有 lastSyncResourceVersion 的 list 
    // 或 watch 请求出现了 “expired” 或 “to large resource version” 错误。
	isLastSyncResourceVersionUnavailable bool
    // lastSyncResourceVersionMutex 保证安全地读写 lastSyncResourceVersion。
	lastSyncResourceVersionMutex sync.RWMutex
    // WatchListPageSize 是 initial 和 resync watch lists 的 chunk size。
    // 如果没有设置，对于持续性的读取 (RV="") 或任意访问旧的数据(RV="0") 将会使用默认值 pager.PageSize。
    // 对于其他的请求（RV != "" && RV != "0"）将会关闭分页功能。
    // NOTE: 当分页的 list 总是被 etcd 直接服务时，需要慎用，因为可能导致严重的性能问题。
	WatchListPageSize int64
	// 无论何时，当 listAndWatch 因为错误丢失链接后都会被调用。
	watchErrorHandler WatchErrorHandler
}
```



### 创建Reflector

```go
// NewReflexor创建一个 Reflector 对象，该对象将使得给定资源的在 store 和 server 端的内容保持最新。
// Reflector 只保证在有 expectedType 的情况下向的 store 中 put 东西，除非 expectedType 为 nil。
// 如果 resyncPeriod 为非零，那么 reflector 将会周期性咨询它的 ShouldResync 函数来决定是否调用
// Store 的重新同步操作；`ShouldResync==nil`意味着总是“yes”。这使您能够使用 Reflector 定期处理所有内容，
// 以及增量处理更改的内容。
func NewReflector(lw ListerWatcher, expectedType interface{}, store Store, resyncPeriod time.Duration) *Reflector {
	return NewNamedReflector(naming.GetNameFromCallsite(internalPackages...), lw, expectedType, store, resyncPeriod)
}
// NewNamespaceKeyedIndexerAndReflector 创建了一个 indexer 和一个 reflector。
func NewNamespaceKeyedIndexerAndReflector(lw ListerWatcher, expectedType interface{}, resyncPeriod time.Duration) (indexer Indexer, reflector *Reflector) {
	indexer = NewIndexer(MetaNamespaceKeyFunc, Indexers{NamespaceIndex: MetaNamespaceIndexFunc})
	reflector = NewReflector(lw, expectedType, indexer, resyncPeriod)
	return indexer, reflector
}
// NewNamedReflector 与 NewReflector 一样，但是可以指定一个名字用于 logging。
func NewNamedReflector(name string, lw ListerWatcher, expectedType interface{}, store Store, resyncPeriod time.Duration) *Reflector {
	realClock := &clock.RealClock{}
	r := &Reflector{
		name:          name,
		listerWatcher: lw,
		store:         store,
		// We used to make the call every 1sec (1 QPS), the goal here is to achieve ~98% traffic reduction when
		// API server is not healthy. With these parameters, backoff will stop at [30,60) sec interval which is
		// 0.22 QPS. If we don't backoff for 2min, assume API server is healthy and we reset the backoff.
		backoffManager:    wait.NewExponentialBackoffManager(800*time.Millisecond, 30*time.Second, 2*time.Minute, 2.0, 1.0, realClock),
		resyncPeriod:      resyncPeriod,
		clock:             realClock,
		watchErrorHandler: WatchErrorHandler(DefaultWatchErrorHandler),
	}
	r.setExpectedType(expectedType)
	return r
}
```

### Reflector 最重要的方法 ListAndWatch()，该方法很长，主要分为 3 个部分，首先 是 List， 然后是周期性的 Resync，最后根据 List 得到的 ResourceVersion 进行 Watch。

```go
// ListAndWatch 首先 list 所有的 items, 并获取对应的 resource version,
// 然后使用 resource version 来 watch 对应的资源.
// 如果 ListAndWatch 没有尝试初始化 watch, 会返回一个错误.
func (r *Reflector) ListAndWatch(stopCh <-chan struct{}) error {
	klog.V(3).Infof("Listing and watching %v from %s", r.expectedTypeName, r.name)
	var resourceVersion string
    // 使用 relistResourceVersion() 获取最新的 ResourceVersion, 并设置到 list 请求的参数中
	options := metav1.ListOptions{ResourceVersion: r.relistResourceVersion()}

	if err := func() error {
		initTrace := trace.New("Reflector ListAndWatch", trace.Field{"name", r.name})
		defer initTrace.LogIfLong(10 * time.Second)
		var list runtime.Object
		var paginatedResult bool
		var err error
		listCh := make(chan struct{}, 1)
		panicCh := make(chan interface{}, 1)
		go func() { 
			defer func() { 
                  // 恢复 panic, 并且将 panic 值传入 panicCh
				if r := recover(); r != nil {
					panicCh <- r
				}
			}()
             // 如果 listerWatcher 支持 chunks, 则尝试在 chunks 中汇聚所有的分页.
			pager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
				return r.listerWatcher.List(opts)
			}))
             // 处理分页
			switch {
			case r.WatchListPageSize != 0:
				pager.PageSize = r.WatchListPageSize
			case r.paginatedResult:
			case options.ResourceVersion != "" && options.ResourceVersion != "0":
				pager.PageSize = 0
			}

			list, paginatedResult, err = pager.List(context.Background(), options)
			if isExpiredError(err) || isTooLargeResourceVersionError(err) {
				r.setIsLastSyncResourceVersionUnavailable(true)
				// Retry immediately if the resource version used to list is unavailable.
				// The pager already falls back to full list if paginated list calls fail due to an "Expired" error on
				// continuation pages, but the pager might not be enabled, the full list might fail because the
				// resource version it is listing at is expired or the cache may not yet be synced to the provided
				// resource version. So we need to fallback to resourceVersion="" in all to recover and ensure
				// the reflector makes forward progress.
				list, paginatedResult, err = pager.List(context.Background(), metav1.ListOptions{ResourceVersion: r.relistResourceVersion()})
			}
			close(listCh)
		}()
		select {
		case <-stopCh:
			return nil
		case r := <-panicCh:
			panic(r)
		case <-listCh:
		}
		if err != nil {
			return fmt.Errorf("failed to list %v: %v", r.expectedTypeName, err)
		}

		// We check if the list was paginated and if so set the paginatedResult based on that.
		// However, we want to do that only for the initial list (which is the only case
		// when we set ResourceVersion="0"). The reasoning behind it is that later, in some
		// situations we may force listing directly from etcd (by setting ResourceVersion="")
		// which will return paginated result, even if watch cache is enabled. However, in
		// that case, we still want to prefer sending requests to watch cache if possible.
		//
		// Paginated result returned for request with ResourceVersion="0" mean that watch
		// cache is disabled and there are a lot of objects of a given type. In such case,
		// there is no need to prefer listing from watch cache.
		if options.ResourceVersion == "0" && paginatedResult {
			r.paginatedResult = true
		}

		r.setIsLastSyncResourceVersionUnavailable(false) // list was successful
		initTrace.Step("Objects listed")
		listMetaInterface, err := meta.ListAccessor(list)
		if err != nil {
			return fmt.Errorf("unable to understand list result %#v: %v", list, err)
		}
		resourceVersion = listMetaInterface.GetResourceVersion()
		initTrace.Step("Resource version extracted")
		items, err := meta.ExtractList(list)
		if err != nil {
			return fmt.Errorf("unable to understand list result %#v (%v)", list, err)
		}
		initTrace.Step("Objects extracted")
         // 调用 syncWith 方法将所有 list 请求得到的数据同步到 DeltaFIFO 中。
		if err := r.syncWith(items, resourceVersion); err != nil {
			return fmt.Errorf("unable to sync list result: %v", err)
		}
		initTrace.Step("SyncWith done")
		r.setLastSyncResourceVersion(resourceVersion)
		initTrace.Step("Resource version updated")
		return nil
	}(); err != nil {
		return err
	}

    // 新启一个 goroutine 执行周期性的 resync操作
	resyncerrc := make(chan error, 1)
	cancelCh := make(chan struct{})
	defer close(cancelCh)
	go func() {
        // resyncChan()方法将创建一个间隔为 resyncPeriod 的定时器，用来周期性执行 resync。
		resyncCh, cleanup := r.resyncChan()
		defer func() {
			cleanup() // Call the last one written into cleanup
		}()
		for {
			select {
			case <-resyncCh:
			case <-stopCh:
				return
			case <-cancelCh:
				return
			}
			if r.ShouldResync == nil || r.ShouldResync() {
				klog.V(4).Infof("%s: forcing resync", r.name)
                  // 将 resync 操作委托给了实现了 store 接口的 DeltaFIFO 进行。
				if err := r.store.Resync(); err != nil {
					resyncerrc <- err
					return
				}
			}
			cleanup()
			resyncCh, cleanup = r.resyncChan()
		}
	}()

	for {
		// give the stopCh a chance to stop the loop, even in case of continue statements further down on errors
		select {
		case <-stopCh:
			return nil
		default:
		}

		timeoutSeconds := int64(minWatchTimeout.Seconds() * (rand.Float64() + 1.0))
		options = metav1.ListOptions{
			ResourceVersion: resourceVersion,
			// We want to avoid situations of hanging watchers. Stop any wachers that do not
			// receive any events within the timeout window.
			TimeoutSeconds: &timeoutSeconds,
			// To reduce load on kube-apiserver on watch restarts, you may enable watch bookmarks.
			// Reflector doesn't assume bookmarks are returned at all (if the server do not support
			// watch bookmarks, it will ignore this field).
			AllowWatchBookmarks: true,
		}

		// start the clock before sending the request, since some proxies won't flush headers until after the first watch event is sent
		start := r.clock.Now()
		w, err := r.listerWatcher.Watch(options)
		if err != nil {
			// If this is "connection refused" error, it means that most likely apiserver is not responsive.
			// It doesn't make sense to re-list all objects because most likely we will be able to restart
			// watch where we ended.
			// If that's the case wait and resend watch request.
			if utilnet.IsConnectionRefused(err) {
				time.Sleep(time.Second)
				continue
			}
			return err
		}
         // 处理 Watch 到的数据
		if err := r.watchHandler(start, w, &resourceVersion, resyncerrc, stopCh); err != nil {
			if err != errorStopRequested {
				switch {
				case isExpiredError(err):
					klog.V(4).Infof("%s: watch of %v closed with: %v", r.name, r.expectedTypeName, err)
				default:
					klog.Warningf("%s: watch of %v ended with: %v", r.name, r.expectedTypeName, err)
				}
			}
			return nil
		}
	}
}
```

watchHanler()处理watch到的变更事件

```go
// watchHandler watches w and keeps *resourceVersion up to date.
func (r *Reflector) watchHandler(start time.Time, w watch.Interface, resourceVersion *string, errc chan error, stopCh <-chan struct{}) error {
	eventCount := 0

	// Stopping the watcher should be idempotent and if we return from this function there's no way
	// we're coming back in with the same watch interface.
	defer w.Stop()

loop:
	for {
		select {
		case <-stopCh: // 停止确认
			return errorStopRequested
		case err := <-errc: // reync 失败, 需要停止 listAndWatch, 重新进行 list.
			return err
		case event, ok := <-w.ResultChan(): // 接收 watch 到的数据
			if !ok {  // 数据不 OK 退出当前 loop
				break loop
			}
			if event.Type == watch.Error { // watch 失败, 记录
				return apierrors.FromObject(event.Object)
			}
			if r.expectedType != nil { // 对于 watch 到的对象类型和 reflector 期望的对象类型是否一致, 如果不一致就跳过.
				if e, a := r.expectedType, reflect.TypeOf(event.Object); e != a {
					utilruntime.HandleError(fmt.Errorf("%s: expected type %v, but watch event object had type %v", r.name, e, a))
					continue
				}
			}
			if r.expectedGVK != nil { // 对比 watch 到对象的 GVK 和 reflector 期望的 GVK 是否一致, 如果不一致就跳过.
				if e, a := *r.expectedGVK, event.Object.GetObjectKind().GroupVersionKind(); e != a {
					utilruntime.HandleError(fmt.Errorf("%s: expected gvk %v, but watch event object had gvk %v", r.name, e, a))
					continue
				}
			}
			meta, err := meta.Accessor(event.Object) // 获取 metadata
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("%s: unable to understand watch event %#v", r.name, event))
				continue
			}
			newResourceVersion := meta.GetResourceVersion() // 获取 resourceVersion, 会将当前的 resourceVersion 更新
		    // 根据 event.Type, 调用 DeltaFIFO 对应的方法, DeltaFIFO 将会将这些事件封装为 Delta 加入队列.
            switch event.Type {  
			case watch.Added:
				err := r.store.Add(event.Object)
				if err != nil {
					utilruntime.HandleError(fmt.Errorf("%s: unable to add watch event object (%#v) to store: %v", r.name, event.Object, err))
				}
			case watch.Modified:
				err := r.store.Update(event.Object)
				if err != nil {
					utilruntime.HandleError(fmt.Errorf("%s: unable to update watch event object (%#v) to store: %v", r.name, event.Object, err))
				}
			case watch.Deleted:
				// TODO: Will any consumers need access to the "last known
				// state", which is passed in event.Object? If so, may need
				// to change this.
				err := r.store.Delete(event.Object)
				if err != nil {
					utilruntime.HandleError(fmt.Errorf("%s: unable to delete watch event object (%#v) from store: %v", r.name, event.Object, err))
				}
			case watch.Bookmark:
				// A `Bookmark` means watch has synced here, just update the resourceVersion
			default:
				utilruntime.HandleError(fmt.Errorf("%s: unable to understand watch event %#v", r.name, event))
			}
			*resourceVersion = newResourceVersion
			r.setLastSyncResourceVersion(newResourceVersion) // 将ResourceVersion 设置为最新的
			eventCount++
		}
	}

	watchDuration := r.clock.Since(start)
    // 所有的 Watch 时长都需要大于 1 s, 不然会产生一个 warning, 被认为是意外关闭.
	if watchDuration < 1*time.Second && eventCount == 0 {
		return fmt.Errorf("very short watch: %s: Unexpected watch close - watch lasted less than a second and no items received", r.name)
	}
	klog.V(4).Infof("%s: Watch close - %v total %v items received", r.name, r.expectedTypeName, eventCount)
	return nil
}
```

## DeltaFIFO

```
DeltaFIFO 是 Reflector 到 Indexer 之间的桥梁.

DeltaFIFO 类似于 FIFO, 但是有两点不同.
1. DeltaFIFO中存储了 Kubernetes 中 对象状态的变化 Delta (Added, Updated, Deleted, Synced),
DeltaFIFO 维护了一个 map, key 为对象生成的键, 值为 Deltas, Deltas 是 Delta对象的切片, 
因为对一个状态的操作会产生很多状态, 因此形成了一个状态序列.
DeltaFIFO 中的对象可以被删除, 删除状态用 DeletedFinalStateUnknown 对象表示.

2. 另一个区别是 DeltaFIFO 有一种额外的方法 Sync, Sync 是同步全量数据到 Indexer.

DeltaFIFO 是一个生产者消费者队列, Reflector 是生产者, 调用 Pop() 方法的为消费者.

DeltaFIFO 适用于这些情况:
1. 每个对象至少会变化(delta)一次.
2. 当处理一个对象时, 想看到上次处理之后该对象发生的所有事.
3. 删除一些对象.
4. 周期性重复处理对象.

 DeltaFIFO 的 `Pop()`, `Get()`, `GetByKey()` 方法返回一个满足 `Store/Queue` 接口的 `interface{}` 对象,
 但是这些方法总是返回一个 Deltas 类型的对象. List() 方法将返回 FIFO 中最新的对象.

 DeltaFIFO 的 `knownObjects KeyListerGetter` 提供了 list Store keys 的能力并且可以通过 Store key
 获取 Objects. 这里 keyListGetter 是一个接口，这里 knownObjects 具体指的是实现了 keyListGetterd 接口的 Indexer 对象.

 关于线程的注意事项:如果从多个线程中并行调用Pop(), 最终可能会有多个线程处理同一个对象的稍微不同的版本.
```

### DeltaFIFO数据结构

```
        ┌───────┐┌───────┐┌───────┐
queue   │ObjKey1││ObjKey2││ObjKey3│
        └───────┘└───────┘└───────┘

        ┌─────────────────────────────────────────────────────────────┐
itmes   │ObjKey1: [{"Added",Obj1} {"Updated",Obj1}]                   │
        ├─────────────────────────────────────────────────────────────┤
        │ObjKey2: [{"Added",Obj2},{"Deleted",Obj2},{"Sync",Obj2}]     │
        ├─────────────────────────────────────────────────────────────┤
        │ObjKey3: [{"Added",Obj3},{"Updated",Obj3},{"Deleted",Obj3}]  │
        └─────────────────────────────────────────────────────────────┘
```

```go

```



```go
type DeltaFIFO struct {
	// lock/cond 保护对 'items' 和 'queue' 的访问.
	lock sync.RWMutex
	cond sync.Cond

    // `items` 是 keys 到 Deltas 的映射.
    // `queue` 维护了 Pop() 消费 FIFO keys 的顺序.
    // keys 在 `items` 和 `queue` 是严格 1:1 的关系, 所有在 `items` 中的 Deltas
    // 都必须有一个 Delta.
	items map[string]Deltas
	queue []string

    // 有两种情况populated 为 true:
    // 1. 通过 Replace 方法插入第一批元素
    // 2. Delete/Add/Updated/AddIfNotPresent 方法被调用.
	populated bool
    // initialPopulationCount 是第一次调用 Replace() 方法插入 items 的数量.
	initialPopulationCount int

    // keyFunc 用来为队列中 item 的插入和提取构造 key, 并且具有确定性(deterministic).
	keyFunc KeyFunc

	// knownObjects 获取的所有对象键。目的是当调用 Replace() 和 Delete() 的时候知道哪一个元素已经被删除
	knownObjects KeyListerGetter

    // 用来表示 queue 已经关闭, 所以当 queue 为空时, 控制循环可以退出.
    // 当前, 并不用来给 CRED 操作设置门槛.
	closed bool

    // emitDeltaTypeReplaced 表示当 Replace() 被调用时, 
    // 是否发布 Replaced 或者 Sync DeltaType(为了向后兼容).
	emitDeltaTypeReplaced bool
}
```





## SharedIndexInformer

上述提到的“Informer有效减轻了Kubernetes的负载”是由SharedIndexInformer负责实现的，Informer是一个接口，Kubernetes对该接口有几个实现，比如SharedInformer，SharedIndexInformer等。其中，使用最多、效率最高的是SharedIndexInformer。

加一个Informer的类图！！！

### 降低APIServer 负载之“Shared”

> 场景：在一个Client中，有多个地方需要感知Kubernetes资源对象的变化，不同的开发者在一个client中可能对同一种资源进行多次实例化。

每次对Informer的实例化都会创建一个与之对应的Reflector，同一个资源在几乎相同的时间点会被多次Decode，这十分消耗CPU资源。

所以Kubernetes提出了SharedInformer(SharedIndexInformer只是多了Indexer)，一种资源在Client中最多只存在一个SharedInformer，可以认为是针对这种资源的“单例“。在Kubernetes中，通过反射类型和map实现同一种资源共享一个SharedInformer：

```go
func NewSharedInformerFactoryWithOptions(client kubernetes.Interface, defaultResync time.Duration, options ...SharedInformerOption) SharedInformerFactory {
	factory := &sharedInformerFactory{
         ...
         // 通过一个map，key为对应资源类型的反射类型 reflect.Type，值为SharedIndexInformer
		informers:        make(map[reflect.Type]cache.SharedIndexInformer),
         ...
	}
    ...
	return factory
}
```

当我们通过sharedInformerFactory.InformerFor()方法创建Informer时，会先去查询是否存在对应资源的sharedIndexInformer，有就直接返回，没有重新创建。

```go
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
    ...

    // 使用 newFunc() 创建新的 informer.
    // newFunc() 为一个函数类型,实际为上一步传入的 f.defaultInformer() 方法
	informer = newFunc(f.client, resyncPeriod)
    // 将创建的 informer 加入 map.
	f.informers[informerType] = informer

	return informer
}
```







![informer架构](img/informer-arch.jpg)

这张图分为两部分, 黄色图标是开发者需要自行开发的部分, 而其它的部分是 client-go 已经提供的, 直接使用即可.
