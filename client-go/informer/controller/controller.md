# controller

源码路径:k8s.io/client-go/tools/cache/controller.go

```go
// 这个文件实现了一个低层级的 controller, 在 sharedIndexInformer 中被使用,
// sharedIndexInformer 是 SharedIndexInformer 接口的一个实现类型.
// 而这些 informers, 则是构成 Kubernetes 控制平面主干的高层控制器的关键组件.
// 例子：
// https://github.com/kubernetes/client-go/tree/master/examples/workqueue
// .

// config封装了Controller大量的重量级方法，如：ListerWatcher，Process，DeltaFIFO等，
// 并在SharedInformer.run方法中进行了初始化。
type Config struct {
    // SharedInformer 使用DeltaFIFO
    // Pop() 方法将会调用 Process() 方法来处理出队的对象.
	Queue 

	// 用来 list 和 watch 资源对象
	ListerWatcher

	// 用来处理 DeltaFIFO.Pop(ProcessFunc) 中出队的 Deltas.
	Process ProcessFunc

    // ObjectType 是一个示例对象的类型, 该类型是当前 controller 期望处理的类型.
    // 只需要类型是正确的, 除非是 `unstructured.Unstructured` 对象,
    // 对象的 apiVersion 和 Kind 也需要正确.
	ObjectType runtime.Object // 在 Reflector 使用

    // FullResyncPeriod 是 ShouldResync 需要考虑的周期
    // 在 Reflector 使用
	FullResyncPeriod time.Duration

    // Reflector 定期使用 ShouldResync 来决定是否重新同步队列。
    // 如果 ShouldResync 是' nil '或返回true，这意味着 Reflector 应该进行 resync。
	ShouldResync ShouldResyncFunc

    // If true, when Process() returns an error, re-enqueue the object.
    // 如果为 true, 当 Process() 返回一个错误, 将会重新将该 object 入队.
    //
	// TODO: add interface to let you inject a delay/backoff or drop
	//       the object completely if desired. Pass the object in
	//       question to this interface as a parameter.  This is probably moot
	//       now that this functionality appears at a higher level.
	RetryOnError bool

    // 当 ListAndWatch 失败时断开连接被调用.
	WatchErrorHandler WatchErrorHandler
}

// ShouldResyncFunc is a type of function that indicates if a reflector should perform a
// resync or not. It can be used by a shared informer to support multiple event handlers with custom
// resync periods.
// ShouldResyncFunc 是一个函数类型, 决定一个 reflector 是否应该 resync.
type ShouldResyncFunc func() bool

// ProcessFunc 处理单个 object.
type ProcessFunc func(obj interface{}) error

```

```go
// Controller 是一个被 Config 参数化的低层次的 controller,
// 在 sharedIndexInformer 中被使用.
type Controller interface {
    // Run 做了两件事:
    // 1. 构建并运行 Reflector, 将 objects/notifications 从 Config 的 ListerWatcher 
    // 发送到 Config 的 Queue中. 也可能在 Queue 上调用 Resync 方法.
    // 2. 反复从 Queue 中 Pop 对象并使用 Config 的 ProcessFunc 处理.
    //
    // 这两件事一直持续知道 `stopCh` 被关闭.
	Run(stopCh <-chan struct{})

    // HasSynced 方法委托给了 Config 的 Queue.
	HasSynced() bool

    // LastSyncResourceVersion 委托给了 Reflector (如果有的话),
    // 否则返回一个空的字符串.
	LastSyncResourceVersion() string
}

// New 基于给定的 Config 创建一个新的 Controller.
func New(c *Config) Controller {
	ctlr := &controller{
		config: *c,
		clock:  &clock.RealClock{},
	}
	return ctlr
}

// `*controller` implements Controller
type controller struct {
	config         Config
	reflector      *Reflector
	reflectorMutex sync.RWMutex
	clock          clock.Clock
}

// Run 开始处理 items, 直到 stopCh 被传入一个值或者被关闭.
// 多次调用 Run() 方法是错误的.
// Run 方法是阻塞的, 需要通过 go 调用.
func (c *controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	go func() {
        // 阻塞, 直到 stopCh 被传入一个值或者被关闭.
        // 然后关闭队列.
		<-stopCh
		c.config.Queue.Close()
    }()
    // 根据 Config 创建一个新的 Reflector.
	r := NewReflector(
		c.config.ListerWatcher,
		c.config.ObjectType,
		c.config.Queue,
		c.config.FullResyncPeriod,
	)
	r.ShouldResync = c.config.ShouldResync
	r.clock = c.clock
	if c.config.WatchErrorHandler != nil {
		r.watchErrorHandler = c.config.WatchErrorHandler
	}

	c.reflectorMutex.Lock()
	c.reflector = r
	c.reflectorMutex.Unlock()

	var wg wait.Group

    // 向 wg 加入一个 r.Run goroutine, stopCh 是 r.Run goroutine 的停止信号
	wg.StartWithChannel(stopCh, r.Run)

	wait.Until(c.processLoop, time.Second, stopCh)
	wg.Wait()
}

// 委托给 Queue.HasSynced(),
// 当 list 的所有对象都已经被 Pop, 返回 true.
func (c *controller) HasSynced() bool {
	return c.config.Queue.HasSynced()
}

// LastSyncResourceVersion 委托给了 Reflector (如果有的话),
// 否则返回一个空的字符串.
func (c *controller) LastSyncResourceVersion() string {
	c.reflectorMutex.RLock()
	defer c.reflectorMutex.RUnlock()
	if c.reflector == nil {
		return ""
	}
	return c.reflector.LastSyncResourceVersion()
}

// processLoop 调用 Queue.Pop 处理 Obj.
// TODO: Consider doing the processing in parallel. This will require a little thought
// to make sure that we don't end up processing the same object multiple times
// concurrently.
//
// TODO: Plumb through the stopCh here (and down to the queue) so that this can
// actually exit when the controller is stopped. Or just give up on this stuff
// ever being stoppable. Converting this whole package to use Context would
// also be helpful.
func (c *controller) processLoop() {
	for {
		obj, err := c.config.Queue.Pop(PopProcessFunc(c.config.Process))
		if err != nil {
			if err == ErrFIFOClosed {
				return
			}
			if c.config.RetryOnError {
                // 重新将对象入队.
                // 但是 Pop 函数中已经有了重试逻辑...???????
				c.config.Queue.AddIfNotPresent(obj)
			}
		}
	}
}
```

```go
// ResourceEventHandler 可以处理发生在一个资源上的事件.
// 这些事件只是起到通知作用, 不能返回 error.
// handler 不能修改接收到的对象.
//  * OnAdd 添加对象时被调用.
//  * OnUpdate 当对象被修改时调用. oldObj 是该对象最后知道的状态 ---- 可能多个变化
//       组合在了一起, 不能使用这个方法辨识每一次变化. OnUpdate 在 re-list 的时候
//       也会被调用, 甚至没有变化也会调用. 这对周期性的评估和验证某些事情非常有用.
//  * OnDelete 会得到对象最后的状态如果知道的话. 否则将会得到一个 DeletedFinalStateUnknown.
//       这可能发生在 watch 被关闭了, 导致 delete 时间丢失, 直到之后的发生了 re-list.
type ResourceEventHandler interface {
	OnAdd(obj interface{})
	OnUpdate(oldObj, newObj interface{})
	OnDelete(obj interface{})
}

// ResourceEventHandlerFuncs是一个适配器，可以让你在实现resourceeventhandler的同时，
// 轻松地指定尽可能多或尽可能少的通知函数。这个适配器并没有取消修改对象的禁令。
type ResourceEventHandlerFuncs struct {
	AddFunc    func(obj interface{})
	UpdateFunc func(oldObj, newObj interface{})
	DeleteFunc func(obj interface{})
}

// OnAdd calls AddFunc if it's not nil.
func (r ResourceEventHandlerFuncs) OnAdd(obj interface{}) {
	if r.AddFunc != nil {
		r.AddFunc(obj)
	}
}

// OnUpdate calls UpdateFunc if it's not nil.
func (r ResourceEventHandlerFuncs) OnUpdate(oldObj, newObj interface{}) {
	if r.UpdateFunc != nil {
		r.UpdateFunc(oldObj, newObj)
	}
}

// OnDelete calls DeleteFunc if it's not nil.
func (r ResourceEventHandlerFuncs) OnDelete(obj interface{}) {
	if r.DeleteFunc != nil {
		r.DeleteFunc(obj)
	}
}
```

```go
// FilteringResourceEventHandler 将会过滤所有的事件, 保证恰当的内置 handler 被调用.
// 与 handler 一样，筛选器也不能修改给定的对象。
// An object that starts passing the filter after an update is considered an add, 
// and an object that stops passing the filter after an update is considered a delete.
type FilteringResourceEventHandler struct {
	FilterFunc func(obj interface{}) bool
	Handler    ResourceEventHandler
}

// OnAdd calls the nested handler only if the filter succeeds
func (r FilteringResourceEventHandler) OnAdd(obj interface{}) {
	if !r.FilterFunc(obj) {
		return
	}
	r.Handler.OnAdd(obj)
}

// OnUpdate ensures the proper handler is called depending on whether the filter matches
func (r FilteringResourceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	newer := r.FilterFunc(newObj)
	older := r.FilterFunc(oldObj)
	switch {
	case newer && older:
		r.Handler.OnUpdate(oldObj, newObj)
	case newer && !older:
		r.Handler.OnAdd(newObj)
	case !newer && older:
		r.Handler.OnDelete(oldObj)
	default:
		// do nothing
	}
}

// OnDelete calls the nested handler only if the filter succeeds
func (r FilteringResourceEventHandler) OnDelete(obj interface{}) {
	if !r.FilterFunc(obj) {
		return
	}
	r.Handler.OnDelete(obj)
}
```

```go
// DeletionHandlingMetaNamespaceKeyFunc 在调用 MetaNamespaceKeyFunc
// 之前检查 obj 是否为 DeletedFinalStateUnknown 类型.
func DeletionHandlingMetaNamespaceKeyFunc(obj interface{}) (string, error) {
	if d, ok := obj.(DeletedFinalStateUnknown); ok {
		return d.Key, nil
	}
	return MetaNamespaceKeyFunc(obj)
}
```

```go
// NewInformer 返回一个 Store 和一个 controller 来填充 store, 同时也提供事件通知.
// 只能用返回的 Store 来进行 Get/List 操作; Add/Modify/Deletes 操作会导致事件通知故障.
// 参数:
// * lw 是你希望被通知的资源的 list 和 watch 函数。
// * objType 是你期望接收的对象类型.
// * resyncPeriod: 如果不为 0, 将会进行 re-list (及时没有变化, 也会收到 OnUpdate Call)
//      否则 re-list 就将尽可能的推迟 (直到上游源头关闭了 watch 或者超时, 或者你关闭了 controller)
//  * h 是处理事件的 handler.
func NewInformer(
	lw ListerWatcher,
	objType runtime.Object,
	resyncPeriod time.Duration,
	h ResourceEventHandler,
) (Store, Controller) {
	// This will hold the client state, as we know it.
	clientState := NewStore(DeletionHandlingMetaNamespaceKeyFunc)

	return clientState, newInformer(lw, objType, resyncPeriod, h, clientState)
}

// NewInformer 返回一个 Indexer 和一个 controller 来填充 index, 同时也提供事件通知.
// 只能用返回的 Index 来进行 Get/List 操作; Add/Modify/Deletes 操作会导致事件通知故障.
//
// 参数:
// * lw 是你希望被通知的资源的 list 和 watch 函数。
// * objType 是你期望接收的对象类型.
// * resyncPeriod: 如果不为 0, 将会进行 re-list (及时没有变化, 也会收到 OnUpdate Call)
//      否则 re-list 就将尽可能的推迟 (直到上游源头关闭了 watch 或者超时, 或者你关闭了 controller)
// * h 是处理事件的 handler.
// * indexers: 用来就收 object type 的 indexer.
func NewIndexerInformer(
	lw ListerWatcher,
	objType runtime.Object,
	resyncPeriod time.Duration,
	h ResourceEventHandler,
	indexers Indexers,
) (Indexer, Controller) {
	// This will hold the client state, as we know it.
	clientState := NewIndexer(DeletionHandlingMetaNamespaceKeyFunc, indexers)

	return clientState, newInformer(lw, objType, resyncPeriod, h, clientState)
}

// newInformer 返回一个 controller 用来填充 store 同时也提供事件通知.
//
// 参数:
// * lw 是你希望被通知的资源的 list 和 watch 函数。
// * objType 是期望接收的对象类型.
// * resyncPeriod: 如果不为 0, 将会进行 re-list (及时没有变化, 也会收到 OnUpdate Call)
//      否则 re-list 就将尽可能的推迟 (直到上游源头关闭了 watch 或者超时, 或者你关闭了 controller)
// * h 是处理事件的 handler.
// * clientState 是期望被填充的 store.
// Parameters
func newInformer(
	lw ListerWatcher,
	objType runtime.Object,
	resyncPeriod time.Duration,
	h ResourceEventHandler,
	clientState Store,
) Controller {
	// This will hold incoming changes. Note how we pass clientState in as a
	// KeyLister, that way resync operations will result in the correct set
    // of update/delete deltas.
    // DeltaFIFO 将会 hold 住到来的修改. 这里传递 clientSet 作为 KeyLister,
    // 
	fifo := NewDeltaFIFOWithOptions(DeltaFIFOOptions{
		KnownObjects:          clientState,
		EmitDeltaTypeReplaced: true,
	})

	cfg := &Config{
		Queue:            fifo,
		ListerWatcher:    lw,
		ObjectType:       objType,
		FullResyncPeriod: resyncPeriod,
		RetryOnError:     false,
        // ProcessFunc 是传入 DeltaFIFO.Pop() 的回调函数,
        // 用于处理 DeltaFIFO 中出队的元素 Deltas (Delta的切片).
		Process: func(obj interface{}) error {
			// from oldest to newest
			for _, d := range obj.(Deltas) {
				switch d.Type {
                case Sync, Replaced, Added, Updated:
                    // 如果对象在本地缓存中已经存在, 则调用 UpdateFunc
					if old, exists, err := clientState.Get(d.Object); err == nil && exists {
						if err := clientState.Update(d.Object); err != nil {
							return err
						}
						h.OnUpdate(old, d.Object)
					} else { // 如果在本地缓存中不存在, 则调用 AddFunc
						if err := clientState.Add(d.Object); err != nil {
							return err
						}
						h.OnAdd(d.Object)
					}
				case Deleted:
					if err := clientState.Delete(d.Object); err != nil {
						return err
					}
					h.OnDelete(d.Object)
				}
			}
			return nil
		},
    }
    // 调用 New() 函数创建 Controller 接口的 controller 实现的实例.
	return New(cfg)
}
```