# Informer

## 简介

在 kubernetes 系统中，组件之间通过 http 协议进行通信，
通过 informer 来做到了消息的实时性、可靠性、顺序性，
通过informer机制与api-server进行通信。
Informer 的机制，降低了了 Kubernetes 各个组件跟 Etcd 与 Kubernetes API Server 的通信压力。
## 架构设计

![informer架构](./img/informer-arch.jpg)

这张图分为两部分，黄色图标是开发者需要自行开发的部分，而其它的部分是 client-go 已经提供的，直接使用即可。

Informer架构设计中有多个核心组件：

1. **Reflector**：
用于 Watch 指定的 Kubernetes 资源，当 watch 的资源发生变化时，触发变更的事件，
比如 Added，Updated 和 Deleted 事件，并将资源对象存放到本地缓存 DeltaFIFO；

2. **DeltaFIFO**：
拆开理解，FIFO 就是一个队列，拥有队列基本方法（ADD，UPDATE，DELETE，LIST，POP，CLOSE 等），
Delta 是一个资源对象存储，保存存储对象的消费类型，比如 Added，Updated，Deleted，Sync 等；

3. **Indexer**：Client-go 用来存储资源对象并自带索引功能的本地存储，
Reflector 从 DeltaFIFO 中将消费出来的资源对象存储到 Indexer，
Indexer 与 Etcd 集群中的数据完全保持一致。
从而 client-go 可以本地读取，减少 Kubernetes API 和 Etcd 集群的压力。

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

	// 创建 stopCh 对象，该对象用于在应用程序退出之前通知 Informer 提前退出
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

### Informer 实现

1. 资源Informer

每一个 Kubernetes 资源都实现了 Informer 机制. 
每一个 Informer 都会实现 Informer 和 List 方法,例如 PodInformer,代码示例如下:
`vendor/k8s.io/client-go/informers/core/v1/pod.go`
```go
// PodInformer provides access to a shared informer and lister for
// Pods.
type PodInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.PodLister
}
```

调用不同资源的 Informer，代码示例如下: 
```go
podInformer := sharedInformers.Core().V1().Pods().Informer()
nodeInformer := sharedInformers.Node().V1beta1().RuntimeClasses().Informer()
```

2. SharedInformer 共享机制

Informer 也被成为 SharedInformer,可以被共享使用.
在使用Client-go编写程序时,若同一个资源的 Informer 被实例化了多次,每个 Informer 都使用一个 Reflector,
那么会运行多个相同的 ListAndWatch,太多重复的序列化和反序列化会导致 Kubernetes API Server 负载过重。

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

## Reflector

Reflector 用于 Kubernetes 资源,当资源发生变化时,触发相应的变更事件,例如添加、更新、删除事件,
并将其资源对象存放到本地缓存 DeltaFIFO 中.

通过 NewReflector 来实例化 Reflector 对象,实例化过程中必须传入 ListerWatcher数据接口对象,
它拥有 List 和 Watch 方法, 用于监控资源列表.
```go
type ListerWatcher interface {
	Lister
	Watcher
}
```

Reflector 实现中,主要是 ListAndWatch 函数,它负责获取资源列表 (List) 和 监控 (Watch) 指定的 Kubernetes API Server 资源.

ListAndWatch 函数实现可以分为两个部分: 1. 获取资源列表数据, 2. 监控资源对象.

```
         r.listerWather.List
                 v
                 v
  listMetaInterface.GetResourceVersion
                 v
                 v
          meta.ExtractList
                 v
                 v
     r.SetLastSyncResourceVersion
```

```go
// ListAndWatch first lists all items and get the resource version at the moment of call,
// and then use the resource version to watch.
// It returns error if ListAndWatch didn't even try to initialize watch.
func (r *Reflector) ListAndWatch(stopCh <-chan struct{}) error {
	klog.V(3).Infof("Listing and watching %v from %s", r.expectedTypeName, r.name)
	var resourceVersion string

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
				if r := recover(); r != nil {
					panicCh <- r
				}
			}()
			// Attempt to gather list in chunks, if supported by listerWatcher, if not, the first
			// list request will return the full response.
			pager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
				return r.listerWatcher.List(opts)
			}))
			switch {
			case r.WatchListPageSize != 0:
				pager.PageSize = r.WatchListPageSize
			case r.paginatedResult:
				// We got a paginated result initially. Assume this resource and server honor
				// paging requests (i.e. watch cache is probably disabled) and leave the default
				// pager size set.
			case options.ResourceVersion != "" && options.ResourceVersion != "0":
				// User didn't explicitly request pagination.
				//
				// With ResourceVersion != "", we have a possibility to list from watch cache,
				// but we do that (for ResourceVersion != "0") only if Limit is unset.
				// To avoid thundering herd on etcd (e.g. on master upgrades), we explicitly
				// switch off pagination to force listing from watch cache (if enabled).
				// With the existing semantic of RV (result is at least as fresh as provided RV),
				// this is correct and doesn't lead to going back in time.
				//
				// We also don't turn off pagination for ResourceVersion="0", since watch cache
				// is ignoring Limit in that case anyway, and if watch cache is not enabled
				// we don't introduce regression.
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

	resyncerrc := make(chan error, 1)
	cancelCh := make(chan struct{})
	defer close(cancelCh)
	go func() {
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

		if err := r.watchHandler(start, w, &resourceVersion, resyncerrc, stopCh); err != nil {
			if err != errorStopRequested {
				switch {
				case isExpiredError(err):
					// Don't set LastSyncResourceVersionUnavailable - LIST call with ResourceVersion=RV already
					// has a semantic that it returns data at least as fresh as provided RV.
					// So first try to LIST with setting RV to resource version of last observed object.
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