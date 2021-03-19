# Reflector

## Reflector 结构

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
	// The GVK of the object we expect to place in the store if unstructured.
    // 期望被放入 store 的对象的 GVK
	expectedGVK *schema.GroupVersionKind
    // 与 watch 资源同步的目的地
	store Store
    // listerWatcher 用来执行 lists 和 watches 操作。
	listerWatcher ListerWatcher

    // backoffManager 用来管理 ListWatch 的执行。
	backoffManager wait.BackoffManager

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
	// WatchListPageSize is the requested chunk size of initial and resync watch lists.
	// If unset, for consistent reads (RV="") or reads that opt-into arbitrarily old data
	// (RV="0") it will default to pager.PageSize, for the rest (RV != "" && RV != "0")
	// it will turn off pagination to allow serving them from watch cache.
	// NOTE: It should be used carefully as paginated lists are always served directly from
	// etcd, which is significantly less efficient and may lead to serious performance and
	// scalability problems.
    // WatchListPageSize 是 initial 和 resync watch lists 的 chunk size。
    // 如果没有设置，对于持续性的读取 (RV="") 或任意访问旧的数据(RV="0") 将会使用默认值 pager.PageSize。
    // 对于其他的请求（RV != "" && RV != "0"）将会关闭分页功能。
    // NOTE: 当分页的 list 总是被 etcd 直接服务时，需要慎用，因为可能导致严重的性能问题。
	WatchListPageSize int64
	// 无论何时，当 listAndWatch 因为错误丢失链接后都会被调用。
	watchErrorHandler WatchErrorHandler
}
```

## WatchErrorHandler

```go

// 无论何时，当 listAndWatch 因为错误丢失链接后都会被调用。
// 当调用此 handler 后，informer 将会 backoff 和 retry。
//
// 默认的实现根据 error type 使用适当的日志等级记录错误信息。
// 实现需要快速返回，任意耗时的处理都应该被去掉。
type WatchErrorHandler func(r *Reflector, err error)

// DefaultWatchErrorHandler 是默认的 WatchErrorHandler 实现。
func DefaultWatchErrorHandler(r *Reflector, err error) {
	switch {
	case isExpiredError(err):
        // 没有设置 LastSyncResourceVersionUnavailable - 带有 ResourceVersion=RV 的 LIST 调用
        // 已经具有一个语义，即它返回的数据至少与提供的RV一样新。
		// So first try to LIST with setting RV to resource version of last observed object.
        // 因此，首先尝试将RV设置为最后观察对象的资源版本。
		klog.V(4).Infof("%s: watch of %v closed with: %v", r.name, r.expectedTypeName, err)
	case err == io.EOF:
		// watch 正常关闭。
	case err == io.ErrUnexpectedEOF:
		klog.V(1).Infof("%s: Watch for %v closed with unexpected EOF: %v", r.name, r.expectedTypeName, err)
	default:
		utilruntime.HandleError(fmt.Errorf("%s: Failed to watch %v: %v", r.name, r.expectedTypeName, err))
	}
}
```

## 创建 Reflector

```go
// NewNamespaceKeyedIndexerAndReflector 创建了一个 indexer 和一个 reflector。
// The indexer is configured to key on namespace
func NewNamespaceKeyedIndexerAndReflector(lw ListerWatcher, expectedType interface{}, resyncPeriod time.Duration) (indexer Indexer, reflector *Reflector) {
	indexer = NewIndexer(MetaNamespaceKeyFunc, Indexers{NamespaceIndex: MetaNamespaceIndexFunc})
	reflector = NewReflector(lw, expectedType, indexer, resyncPeriod)
	return indexer, reflector
}
```

```go
// NewReflexor创建一个 Reflector 对象，该对象将使得给定资源的在 store 和 server 端的内容保持最新。
// Reflector 只保证在有 expectedType 的情况下向的 store 中 put 东西，除非 expectedType 为 nil。
// 如果 resyncPeriod 为非零，那么 reflector 将会周期性咨询它的 ShouldResync 函数来决定是否调用
// Store 的重新同步操作；`ShouldResync==nil`意味着总是“yes”。这使您能够使用 Reflector 定期处理所有内容，
// 以及增量处理更改的内容。
func NewReflector(lw ListerWatcher, expectedType interface{}, store Store, resyncPeriod time.Duration) *Reflector {
	return NewNamedReflector(naming.GetNameFromCallsite(internalPackages...), lw, expectedType, store, resyncPeriod)
}
```

```go
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

## ListAndWatch

```go
// ListAndWatch 首先 list 所有的 items, 并获取对应的 resource version,
// 然后使用 resource version 来 watch 对应的资源.
// 如果 ListAndWatch 没有尝试初始化 watch, 会返回一个错误.
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

```go
// relistResourceVersion determines the resource version the reflector should list or relist from.
// Returns either the lastSyncResourceVersion so that this reflector will relist with a resource
// versions no older than has already been observed in relist results or watch events, or, if the last relist resulted
// in an HTTP 410 (Gone) status code, returns "" so that the relist will use the latest resource version available in
// etcd via a quorum read.
// relistResourceVersion 决定 Reflector 应用 list 还是 relist 资源版本（resource version）.
// 返回以下两种情况中的一个：
// 1. 返回 lastSyncResourceVersion, 以便于当前 Reflector relist 不早于 resource version 的已经在 relist 结果
//    或 watch event 中被观察到的 resource version. 也就是说返回 lastSyncResourceVersion, 即上一次观察到的版本,
//    以便于重新从上一次开始 relist.
// 2. 如果上一次 relist 结果返回了 HTTP 410 (gone) 状态码, 则返回 "" 以便于 relist 使用 最新的
//    可用的 resource version（通过 etcd quorum read 获取）.
func (r *Reflector) relistResourceVersion() string {
	r.lastSyncResourceVersionMutex.RLock()
	defer r.lastSyncResourceVersionMutex.RUnlock()

	if r.isLastSyncResourceVersionUnavailable {
		// Since this reflector makes paginated list requests, and all paginated list requests skip the watch cache
		// if the lastSyncResourceVersion is unavailable, we set ResourceVersion="" and list again to re-establish reflector
		// to the latest available ResourceVersion, using a consistent read from etcd.
		return ""
	}
	if r.lastSyncResourceVersion == "" {
		// For performance reasons, initial list performed by reflector uses "0" as resource version to allow it to
		// be served from the watch cache if it is enabled.
		return "0"
	}
	return r.lastSyncResourceVersion
}
```

