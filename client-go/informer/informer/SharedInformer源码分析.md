# SharedInformer 源码分析

## SharedInformer 介绍

`SharedInformer` 提供了客户端与给定对象集合的权威状态(authoritative state)的最终一致链接.

一个对象由其 `API group, kind/resource, namespace(如果有的话), name` 确定; 
这里, `ObjectMeta.UID` 并不是一个 `object` 的 `ID` 的一部分.

一个 `SharedInformer` 为一个特定 `API group` 和 `kind/resource` 提供了连接.
一个 `SharedInformer` 连接的对象集合可能会被 `namespace` 或 `label selector` 或 field selector` 限制.

---

一个对象的权威状态(authoritative state)是访问 apiserver 提供的, 并且一个对象会经历一系列严格的状态. 
一个对象的状态要么是(1)呈现 `ResourceVersion` 和其他适当内容，要么是(2)"absent"。

---

一个 `SharedInformer` 维护了一个存储相关对象状态的本地缓存 ---- 通过 `GetStore()` 暴露; 如果是 `Indexed Informer`, 则通过 `GetIndexer()` 暴露; 也可能被包含创建/访问` informer` 的 machinery 暴露. 这个缓存与权威状态(authoritative state)保持最终一致性. 

除非遇到持续的通信问题, 如果有一个特定对象的 `ID X` 被权威关联了一个状态 `S`, 对于每一个包含 `(X, S) `的集合的 `SharedInformer I`, 最终只存在以下三种情况之一:
- `I` 的缓存中 `X` 关联了 状态`S`, 或者关联了 `X` 之后的状态;
- `I` 已经被停止;
- `X` 的权威状态服务(authoritative state service)被已经关闭.

状态 "absent" 满足标签选择器和字段选择器的任意限制.

---

==对于给定的` informer` 和相关的对象`ID X`,  `informer` 缓存中出现的状态序列是与` X` 权威关联的状态的子序列, 也就是说, 有些状态可能永远不会出现在缓存中, 但出现状态之间的顺序是正确的. 但是请注意，并不保证不同对象的状态之间的顺序.==

---

本地缓存一开始是空的, 在 `Run()` 方法执行期间填充和更新.

---

举一个简单的例子, 对于一个不再改变的对象集合, 创建一个 `SharedInformer` 连接集合, `SharedInformer` 运行以后, `SharedInformer` 缓存最终会持有一个精确复制的集合(除非它被很快停止, 或权威状态服务结束, 或两者之间被持久的通讯问题阻扰)。

另外一个例子, 如果本地缓存中某些 `object ID` 有过 `non-absent` 状态, 并且 这个 `object` 最终从 权威状态中删除, 那么 这个 `object` 最终也会在本地缓存中删除(除非 SharedInformer 被很快停止, 权威状态服务关闭, 或者持续的通信问题扰乱了期望的状态)

---

==`Store` 中的键(keys) 有两种形式:== 

- ==对于有命名空间的对象(`namespaced objects`)是 `namespace/name`,== 
- ==没有命名空间的对象(`non-namespaced objects`)是` name`.==

 ==Client 可以使用 `MetaNamespaceKeyFunc(obj)` 来提取一个给定 object 的 key, 以及 `SplitMetaNamespaceKey(key)` 将一个 key 分成其组成的部分.==

---

每一个对本地缓存的查询都是由某个缓存状态的 snapshot 响应. 因此, `List` 调用将不会包含拥有相同 `namespace` 和 `name` 的两条记录.

---

 一个 client 被一个 `ResourceEventHandler` 识别. 
- 对于所有对 SharedInformer 本地缓存的修改, 以及每个在 `Run` 执行之前被加入的 client, 最终要么 SharedInformer 被停止, 要么 SharedInformer 收到了修改的通知. 
- 对于 `Run()` 执行后添加的 client, 当其被添加到 SharedInformer 时, 会收到一批缓存中已存在对象的添加通知. 同样, client 被添加后, 所有对 SharedInformer 本地缓存的修改, 最终要么是 SharedInformer 被停止, 或者 client 收到了修改通知.

客户端通知在相应的缓存更新之后发生，对于 SharedIndexInformer，在相应的索引更新之后发生. 额外的缓存和索引更新可能发生在这样一个规定的通知之前. 

对于一个给定的 SharedInformer 和 client, 通知是顺序(sequentially)投递的.
对于一个给定的 SharedInformer, client 和 object ID, 通知是按顺序(in order)投递的.

因为, `ObjectMeta.UID` 对于识别 objects 没有任何作用, 可能当
(1) object O1 (`ID: X`, `ObjectMeta.UID: U1`) 在 SharedInformer 的缓存中被删除, 
然后 (2) 另外一个 object O2 (`ID: X`, `ObjectMeta.UID: U2`) 被创建, 并且 informer 的 client 没有收到通知(1)和(2), 只收到一个 O1 到 O2 的更新通知. 
客户端需要侦测这种情况, 可能会在处理 upadate 通知时, 在代码中比较新旧两个对象的 `ObjectMeta.UID` 字段, 比如在 `ResourceEventHandler` 中 `OnUpdate` 方法中对比 UID.

---

一个客户端必须合理的处理通知, SharedInformer 不是为了处理大量积压的通知而设计的. 长时间的处理应该使用其他手段, 比如 `client-go/util/workqueue`.

---

delete 通知会暴露最后一个本地已知的非缺席状态, 只不过它的ResourceVersion 被替换为实际上不存在对象的 ResourceVersion.

---

### SharedInformer 接口

`SharedInformer` 接口定义了其需要共享的 informer 需要实现的方法.


```go
type SharedInformer interface {
    // AddEventHandler 使用 shared informer 的 resync period 向 shared informer 添加一个事件处理程序.
    // 单个 handler 的事件是按顺序传递的，但是不同 handler 之间没有协作。
	AddEventHandler(handler ResourceEventHandler)
    // AddEventHandlerWithResyncPeriod 在给定重新同步周期(resync period) 向 shard informer 添加 event handler.
    // resyncPeriod 为 0 代表 handler 并不关注重新同步(resync).
    // 重新同步操作包括向 informer 的本地缓存中所有对象投递一条 update 通知,
    // 并不会和 authoritative storage 有任何交互.
    // 某些 informer 并不重新同步, 对设置了 non-zero resync 的 handler 也是这样.
    // 对与不重新同步的 informer 和要求重新同步的每个 handler,
    // 这种 informer 开发了一个名义上的重新同步周期(resync period),
    // 这个同步周期不短于给定的 resyncPeriod.
    // 实际两次重新同步的时间要比名义上的 period 要长一些, 因为实现会花费一部分时间, 
    // 以及计算负载和调度都需要花费时间.
	AddEventHandlerWithResyncPeriod(handler ResourceEventHandler, resyncPeriod time.Duration)
    // GetStore 返回 informer 的本地缓存作为 Store.
	GetStore() Store
    // 这个方法已经废弃.
	GetController() Controller
    // Run 启动和运行一个 informer, 知道 informer 停止才返回.
    // 当 stopCh 关闭, informer 就会停止.
	Run(stopCh <-chan struct{})
	// 这个函数用来告知调用者 Store 本地缓存是否已经同步了 apiserver 的资源.
	// 当 SharedInformer 创建完毕之后, 通过 Reflector 从 apiserver 全量同步(list)对象,
	// 然后通过 DeltaFIFO 通知本地缓存. 这个接口就是告知使用者，全量的对象是不是已经同步到了cache，
	// 这样就可以从cache列举或者查询了.
	HasSynced() bool
    // LastSyncResourceVersion 是上一次和 store 同步观察到的 resource version.
	// 返回值并不是和访问依赖的 store 同步的, 并且不是线程安全的.
	// 最新同步资源的版本, 通过Controller(Controller 通过 Reflector)实现.
	LastSyncResourceVersion() string
    // WatchErrorHandler 在 ListAndWatch 因为错误释放连接时被调用.
    // 当这个 handler 被调用时, informer 将会回滚和重试.
    //
    // 默认的实现会记录错误类型和尝试使用一个适当的错误级别记录错误信息.
    //
    // 只能存在一个 handler, 如果调用了这个方法多次, 会设置最后一个 handler,
    // 如果在 informer 启动之后调用, 将会返回一个错误.
	SetWatchErrorHandler(handler WatchErrorHandler) error
}
```

### SharedIndexInformer 接口

```go
// SharedIndexInformer 基于 SharedInformer 提供了 add 和 get indexer 的能力.
type SharedIndexInformer interface {
	SharedInformer
    // AddIndexers 在 informer 启动前向其添加 indexers.
	AddIndexers(indexers Indexers) error
	GetIndexer() Indexer
}
```

### 创建SharedInformer和SharedIndexInformer

使用 `NewSharedInformer` 创建 `SharedInformer`. 实际上, 使用了 `NewSharedIndexInformer` 创建了一个 `SharedIndexInformer`, 其 `indexer` 字段为 `Indexer{}`.

```go
// NewSharedInformer 创建了一个新的 listwatcher 实例.
func NewSharedInformer(lw ListerWatcher, exampleObject runtime.Object, defaultEventHandlerResyncPeriod time.Duration) SharedInformer {
    // NewSharedInformer 实际调用 NewSharedIndexInformer 创建了一个 indexer 参数为 Indexer{} 的 SharedIndexInformer.
	return NewSharedIndexInformer(lw, exampleObject, defaultEventHandlerResyncPeriod, Indexers{})
}
```

使用 NewSharedIndexerInformer 创建 SharedIndexInformer.

```go
// NewSharedIndexInformer 创建了一个新的 listwatcher 实例.
// 如果参数 defaultEventHandlerResyncPeriod 为 0,  则 informer 不会执行resync 操作. 
// 否则, 对于每一个重新同步周期不为 0 的 handler,  不管在 informer 启动前还是启动后添加的, 
// 其名义上的重新同步周期四舍五入后是该 informer 重新同步检查周期(informer's resync checking period)的倍数.
// 重新同步检查周期在 informer 启动运行时被建立, 是 (a) 和 (b) 之间的最大值:
// (a) informer 启动前请求的 resync period 和 defaultEventHandlerResyncPeriod
// 的最小值, (b) 当前文件中 `minimumResyncPeriod` 定义的值(1s).
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

## 同步函数

### InformerSynced

```go
// InformerSynced 可以用来判断一个 informer 是否已经同步.
// 这对于判断缓存是否同步十分有用.
type InformerSynced func() bool
```

## 同步操作定义的常量

```go
const (
  // syncedPollPeriod 用来控制查询你的同步函数(sync funcs)状态的周期
	syncedPollPeriod = 100 * time.Millisecond
  // initialBufferSize 是事件通知能被缓冲的初始数量.
	initialBufferSize = 1024
)
```

### 等待缓存同步 WaitForNamedCacheSync 和 WaitForCacheSync

WaitForNamedCacheSync 对 WaitForCacheSync 进行了包装, 在调用 WaitForCacheSync 前打印日志消息——正在等待缓存同步的 controllerName. 在调用 WaitForCacheSync 后打印日志消息——该 controller 缓存同步是否成功.

```go
// WaitForNamedCacheSync 是 WaitForCacheSync 的包装器, 它生成日志消息, 
// 显示由名称(name)标识的调用者正在等待同步, 然后显示同步是否成功.
func WaitForNamedCacheSync(controllerName string, stopCh <-chan struct{}, cacheSyncs ...InformerSynced) bool {
	klog.Infof("Waiting for caches to sync for %s", controllerName)

	if !WaitForCacheSync(stopCh, cacheSyncs...) {
		utilruntime.HandleError(fmt.Errorf("unable to sync caches for %s", controllerName))
		return false
	}

	klog.Infof("Caches are synced for %s ", controllerName)
	return true
}
```

WaitForCacheSync

```go
// WaitForCacheSync 等待缓存同步. 如果成功, 返回true, 如果 返回 false, 则 controller 应该关闭.
// 调用者更应该选择 WaitForNamedCacheSync().
func WaitForCacheSync(stopCh <-chan struct{}, cacheSyncs ...InformerSynced) bool {
    // 使用 wait.PollImmediateUntil 周期性的检验所有 Informer 中的缓存是否同步.
	err := wait.PollImmediateUntil(syncedPollPeriod,
        // 这里的 condition func 需要所有 cacheSyncs 中的 InformerSynced 函数都返回 true 才会返回 true.
		func() (bool, error) {
			for _, syncFunc := range cacheSyncs {
				if !syncFunc() {
					return false, nil
				}
			}
			return true, nil
		},
		stopCh)
	if err != nil {
		klog.V(2).Infof("stop requested")
		return false
	}

	klog.V(4).Infof("caches populated")
	return true
}
```

## SharedIndexInformer

### SharedIndexInformer 结构

```go
// sharedIndexInformer 实现了 SharedIndexInformer 接口.
// sharedIndexInformer 有三大组件: 
// 1. Indexer: 带索引的本地缓存
// 2. Controller: 使用 ListerWatcher 拉取 objects/notifications,
//    然后将其推送到 DeltaFIFO. DeltaFIFO 的 knownObject 是 informer 的
//    的本地缓存 ---- 并发地从 fifo 中弹出 Deltas, 然后使用
//    sharedIndexInformer::HandleDeltas 进行处理. 每一次调用 HandleDeltas
//    都会获取 fifo 的锁, 顺序处理每个 Delta. 每个 Delta 都会更新本地缓存, 并且
//    将相关的消息发送到 sharedProcessor.
// 3. sharedProcessor: 负责消息发送到每个 informer 的 clients.
type sharedIndexInformer struct {
	indexer    Indexer

	controller Controller

	processor             *sharedProcessor
	cacheMutationDetector MutationDetector

	listerWatcher ListerWatcher

    // objectType 是当前示例对象(example object)的对象类型, 表示被当前 informer 处理的 object 的类型.
    // 只需保证类型是正确的, 除非是 `unstructured.Unstructured` 类型,
    // 该对象的 `apiVersion` 和 `kind` 必须正确.
	objectType runtime.Object

    // resyncCheckPeriod 是期望启动 reflector 的 resync timer 的周期, 
    // 这样就可以检查我们的 listeners 是否需要重新同步.
	resyncCheckPeriod time.Duration
    // defaultEventHandlerResyncPeriod 是任意的通过 AddEventHanler 添加的 handler 的 resync period.
    // (比如, 不指定 resync period, 使用 shared informer 的默认值)
	defaultEventHandlerResyncPeriod time.Duration
    // 具有测试能力的时钟
	clock clock.Clock

	started, stopped bool
	startedLock      sync.Mutex

    // blockDeltas 将暂停所有的事件分发, 为了使得一个 event handler 可以被安全地加入 shared informer.
	blockDeltas sync.Mutex

	// Called whenever the ListAndWatch drops the connection with an error.
    // 无论何时, ListAndWatch 因为错误断开连接, 都会调用 watchErrorHandler
	watchErrorHandler WatchErrorHandler
}
```

### Run

SharedInformer 是万恶之源，先后构建了NewDeltaFIFO，Controller，HandleDeltas，
sharedProcessor->processorListener处理器，并最后驱动Controller.run。

```go
func (s *sharedIndexInformer) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	fifo := NewDeltaFIFOWithOptions(DeltaFIFOOptions{
		KnownObjects:          s.indexer,
		EmitDeltaTypeReplaced: true,
	})

	cfg := &Config{
		Queue:            fifo,
		ListerWatcher:    s.listerWatcher,
		ObjectType:       s.objectType,
		FullResyncPeriod: s.resyncCheckPeriod,
		RetryOnError:     false,
		ShouldResync:     s.processor.shouldResync,
		// controller 的 ProcesFunc 就是传入 DeltaFIFO.Pop() 的回调函数,
		// 用来处理每一个出队的 Deltas (Delta 的切片)
		Process:           s.HandleDeltas,
		WatchErrorHandler: s.watchErrorHandler,
	}

	// 创建 controller
	func() {
		s.startedLock.Lock()
		defer s.startedLock.Unlock()

		s.controller = New(cfg)
		s.controller.(*controller).clock = s.clock
		s.started = true
	}()

	// 分开停止channel, 因为 Processor 和应该严格的在 controller 之后停止.
	processorStopCh := make(chan struct{})
	var wg wait.Group
	defer wg.Wait()              // Wait for Processor to stop
	defer close(processorStopCh) // Tell Processor to stop
	wg.StartWithChannel(processorStopCh, s.cacheMutationDetector.Run)
	wg.StartWithChannel(processorStopCh, s.processor.run)

	defer func() {
		s.startedLock.Lock()
		defer s.startedLock.Unlock()
		s.stopped = true // Don't want any new listeners
	}()
	s.controller.Run(stopCh)
}
```

### HasSynced and LastSyncResourceVersion

```go
func (s *sharedIndexInformer) HasSynced() bool {
	s.startedLock.Lock()
	defer s.startedLock.Unlock()

	if s.controller == nil {
		return false
	}
	return s.controller.HasSynced()
}

func (s *sharedIndexInformer) LastSyncResourceVersion() string {
	s.startedLock.Lock()
	defer s.startedLock.Unlock()

	if s.controller == nil {
		return ""
	}
	return s.controller.LastSyncResourceVersion()
}
```

### Adder and Getter

```go
func (s *sharedIndexInformer) GetStore() Store {
	return s.indexer
}

func (s *sharedIndexInformer) GetIndexer() Indexer {
	return s.indexer
}

func (s *sharedIndexInformer) AddIndexers(indexers Indexers) error {
	s.startedLock.Lock()
	defer s.startedLock.Unlock()

	if s.started {
		return fmt.Errorf("informer has already started")
	}

	return s.indexer.AddIndexers(indexers)
}

// GetController 返回一个 `dummyController`, dummyController 
// 封装了 `sharedIndexInformer`, 只对外暴露了 sharedIndexInformer 的 `HasSynced` 方法. 
func (s *sharedIndexInformer) GetController() Controller {
	return &dummyController{informer: s}
}

// 只能在 informer 启动前添加 WatchErrorHandler.
// 多次添加, 以最后一次为准.
func (s *sharedIndexInformer) SetWatchErrorHandler(handler WatchErrorHandler) error {
	s.startedLock.Lock()
	defer s.startedLock.Unlock()

	if s.started {
		return fmt.Errorf("informer has already started")
	}

	s.watchErrorHandler = handler
	return nil
}
```

### 添加 Handler
```go
func (s *sharedIndexInformer) AddEventHandler(handler ResourceEventHandler) {
	s.AddEventHandlerWithResyncPeriod(handler, s.defaultEventHandlerResyncPeriod)
}

`AddEventHandlerWithResyncPeriod` 可以向 informer 添加带 `resyncPeriod` 的 handler.

const minimumResyncPeriod = 1 * time.Second

- 不能向停止的 informer 添加 handler.
- handler 的 resyncPeriod 不能小于 minimumResyncPeriod, 如果小于, 则按 minimumResyncPeriod 处理.
- 如果 informer 已经启动, 并且 handler 的 resyncPeriod 小于 当前 informer 的 resyncCheckPeriod,
则将 resyncPeriod 更新为 resyncCheckPeriod.
- 如果 informer 未启动, 并且 handler 的 resyncPeriod 小于 当前 informer 的 resyncCheckPeriod,
- 则更新当前 informer 的 resyncCheckPeriod 为 resyncPeriod, 并且将之前设置 listener 的 resync periods
全部设置为 resyncPeriod.
func (s *sharedIndexInformer) AddEventHandlerWithResyncPeriod(handler ResourceEventHandler, resyncPeriod time.Duration) {
	s.startedLock.Lock()
	defer s.startedLock.Unlock()

    // 不能向停止的 informer 添加 handler.
	if s.stopped {
		klog.V(2).Infof("Handler %v was not added to shared informer because it has stopped already", handler)
		return
	}

	if resyncPeriod > 0 {
        // resyncPeriod 不能小于 minimumResyncPeriod
		if resyncPeriod < minimumResyncPeriod {
			klog.Warningf("resyncPeriod %d is too small. Changing it to the minimum allowed value of %d", resyncPeriod, minimumResyncPeriod)
			resyncPeriod = minimumResyncPeriod
		}
        // resyncPeriod 不能小于当前 informer 中定义的 resyncCheckPeriod
		if resyncPeriod < s.resyncCheckPeriod {
			if s.started {
                // 如果 informer 已经启动, 则使用用当前 informer 定义的 resyncCheckPeriod
				klog.Warningf("resyncPeriod %d is smaller than resyncCheckPeriod %d and the informer has already started. Changing it to %d", resyncPeriod, s.resyncCheckPeriod, s.resyncCheckPeriod)
				resyncPeriod = s.resyncCheckPeriod
			} else {
                // 如果当前 informer 没有启动, 同时 resyncPeriod 小于当前 informer 的 resyncCheckPeriod,
                // 则更新 resyncCheckPeriod 为 resyncPeriod, 并调整所有 listener 的 resync periods.
				s.resyncCheckPeriod = resyncPeriod
				s.processor.resyncCheckPeriodChanged(resyncPeriod)
			}
		}
	}

    // 创建一个新的 listener
	listener := newProcessListener(handler, resyncPeriod, determineResyncPeriod(resyncPeriod, s.resyncCheckPeriod), s.clock.Now(), initialBufferSize)

    // 如果 informer 没有启动,  直接添加 listener.
	if !s.started {
		s.processor.addListener(listener)
		return
	}

    // 为了在 informer 启动后安全地添加 handler, 需要:
    // 0. 加锁
    // 1. 暂定发送 add/update/delete 消息.
    // 2. 向 store 发一个 list 请求, 得到所有的 item.
    // 3. 向新的 handler 发送 list 中所有 item 的 "Add" 事件.
    // 4. 释放锁.
	s.blockDeltas.Lock()
	defer s.blockDeltas.Unlock()

	s.processor.addListener(listener)
	for _, item := range s.indexer.List() {
		listener.add(addNotification{newObj: item})
	}
}
```

### handleDelta

```go
func (s *sharedIndexInformer) HandleDeltas(obj interface{}) error {
	s.blockDeltas.Lock()
	defer s.blockDeltas.Unlock()

	// from oldest to newest
	for _, d := range obj.(Deltas) {
		switch d.Type {
		case Sync, Replaced, Added, Updated:
			s.cacheMutationDetector.AddObject(d.Object)
			if old, exists, err := s.indexer.Get(d.Object); err == nil && exists {
				if err := s.indexer.Update(d.Object); err != nil {
					return err
				}

				isSync := false
				switch {
				case d.Type == Sync:
					// Sync events are only propagated to listeners that requested resync
					isSync = true
				case d.Type == Replaced:
					if accessor, err := meta.Accessor(d.Object); err == nil {
						if oldAccessor, err := meta.Accessor(old); err == nil {
							// Replaced events that didn't change resourceVersion are treated as resync events
							// and only propagated to listeners that requested resync
							isSync = accessor.GetResourceVersion() == oldAccessor.GetResourceVersion()
						}
					}
				}
				s.processor.distribute(updateNotification{oldObj: old, newObj: d.Object}, isSync)
			} else {
				if err := s.indexer.Add(d.Object); err != nil {
					return err
				}
				s.processor.distribute(addNotification{newObj: d.Object}, false)
			}
		case Deleted:
			if err := s.indexer.Delete(d.Object); err != nil {
				return err
			}
			s.processor.distribute(deleteNotification{oldObj: d.Object}, false)
		}
	}
	return nil
}
```

## dummyController

```go
// dummyController 隐藏了 SharedInformer 可以被调用者 `Run` 的事实, 可以看到 dummyController 
// 拥有一个 *sharedIndexInformer 类型的对象, 但是 Run 方法没有做任何事情. 上层逻辑将会决定什么时候
// 启动 SharedInformer 和 相关的 controller.
// 
// Because returning information back is always asynchronous, the legacy callers shouldn't
// notice any change in behavior.


type dummyController struct {
	informer *sharedIndexInformer
}

func (v *dummyController) Run(stopCh <-chan struct{}) {
}

func (v *dummyController) HasSynced() bool {
	return v.informer.HasSynced()
}

func (v *dummyController) LastSyncResourceVersion() string {
	return ""
}
```

### SharedProcessor
通过SharedInformer.AddEventHandler()添加的处理器最终就会封装成processorListener，
然后通过sharedProcessor管理起来，通过processorListener的封装就可以达到所谓的有事处理，
没事挂起。

```go
// sharedProcessor 维护了一组 processorListener, 也用来分发通知到相关的 listeners.
// 有两种类型的分发造作:
// 1. sync distribution: 只分发到重新同步的 listeners 和新加入的 liseners.
// 2. non-sync distribution: 分发到每一个 listener.
type sharedProcessor struct {
	listenersStarted bool
	listenersLock    sync.RWMutex
	listeners        []*processorListener
	syncingListeners []*processorListener
	clock            clock.Clock
	wg               wait.Group
}

func (p *sharedProcessor) addListener(listener *processorListener) {
	p.listenersLock.Lock()
	defer p.listenersLock.Unlock()

	p.addListenerLocked(listener)
	if p.listenersStarted {
		p.wg.Start(listener.run)
		p.wg.Start(listener.pop)
	}
}

func (p *sharedProcessor) addListenerLocked(listener *processorListener) {
	p.listeners = append(p.listeners, listener)
	p.syncingListeners = append(p.syncingListeners, listener)
}

func (p *sharedProcessor) distribute(obj interface{}, sync bool) {
	p.listenersLock.RLock()
	defer p.listenersLock.RUnlock()

	if sync {
		for _, listener := range p.syncingListeners {
			listener.add(obj)
		}
	} else {
		for _, listener := range p.listeners {
			listener.add(obj)
		}
	}
}

func (p *sharedProcessor) run(stopCh <-chan struct{}) {
	func() {
		p.listenersLock.RLock()
		defer p.listenersLock.RUnlock()
		for _, listener := range p.listeners {
			p.wg.Start(listener.run)
			p.wg.Start(listener.pop)
		}
		p.listenersStarted = true
	}()
	<-stopCh
	p.listenersLock.RLock()
	defer p.listenersLock.RUnlock()
	for _, listener := range p.listeners {
		close(listener.addCh) // Tell .pop() to stop. .pop() will tell .run() to stop
	}
	p.wg.Wait() // Wait for all .pop() and .run() to stop
}

// shouldResync queries every listener to determine if any of them need a resync, based on each
// listener's resyncPeriod.
func (p *sharedProcessor) shouldResync() bool {
	p.listenersLock.Lock()
	defer p.listenersLock.Unlock()

	p.syncingListeners = []*processorListener{}

	resyncNeeded := false
	now := p.clock.Now()
	for _, listener := range p.listeners {
		// need to loop through all the listeners to see if they need to resync so we can prepare any
		// listeners that are going to be resyncing.
		if listener.shouldResync(now) {
			resyncNeeded = true
			p.syncingListeners = append(p.syncingListeners, listener)
			listener.determineNextResync(now)
		}
	}
	return resyncNeeded
}

func (p *sharedProcessor) resyncCheckPeriodChanged(resyncCheckPeriod time.Duration) {
	p.listenersLock.RLock()
	defer p.listenersLock.RUnlock()

	for _, listener := range p.listeners {
		resyncPeriod := determineResyncPeriod(listener.requestedResyncPeriod, resyncCheckPeriod)
		listener.setResyncPeriod(resyncPeriod)
	}
}
```

## ProcessorListener

```go
// processorListener 将通知从 sharedProcessor 传递到一个 ResourceEventHandler.
// 过程使用了两个 goroutine, 两个 unbuffered channels 和一个 unbounded ring buffer.
// `add(notification)` 函数发送该通知到 `addCh`. 一个 goroutine 运行 `Pop()`,
// 使用 ring buffer 中的 storage 将通知从 `addCh` 去处并发送到 `nextCh`
// 另外一个 goroutine 运行 `run`, 从 `nextCh` 接收通知并且同步调用适当的 handler.
//
// processorListener 会跟踪调整后的 listener 请求的重新同步周期。
type processorListener struct {
	nextCh chan interface{}
	addCh  chan interface{}

	handler ResourceEventHandler

	// pendingNotificatoins 是一个 unbounded ring buffer 用来存储所有当前还未被分发的通知.
	// 失败的/暂定的 listener 将会有一个无限多个 pendingNotifications 被添加直到 OOM
	// 
	// TODO: This is no worse than before, since reflectors were backed by unbounded DeltaFIFOs, but
	// we should try to do something better.
	pendingNotifications buffer.RingGrowing

	// requestedResyncPeriod 是 listener 和 shared informer 全量重新同步的周期,
	// 收两个因素影响. 一个是设置下限`minimumResyncPeriod`.
	// 另一个是另一个下限，sharedProcessor的 `resyncCheckPeriod`, 
	// 它(a)只在sharedProcessor启动后调用 AddEventHandlerWithResyncPeriod 时生效,
	// (b)只有当informer重新同步时才生效.
	requestedResyncPeriod time.Duration

	// resyncPeriod 是当前 listener 逻辑使用上使用的阈值.
	// 只有当 sharedIndexInformer 不重新同步时, 这个值和 requestedResyncPeriod 不同,
	// 即当 resyncPeriod 为 0 时. 真实的重新同步由 sharedProcessor 的 `shouldResync`方法
	// 何时被调用和当 sharedIndexInformer 处理 `sync` 类型的 Delta 对象决定.
	resyncPeriod time.Duration
	// nextResync is the earliest time the listener should get a full resync
	// nextResync 是
	nextResync time.Time
	// resyncLock guards access to resyncPeriod and nextResync
	resyncLock sync.Mutex
}

// 创建 ProcessListener
func newProcessListener(handler ResourceEventHandler, requestedResyncPeriod, resyncPeriod time.Duration, now time.Time, bufferSize int) *processorListener {
	ret := &processorListener{
		nextCh:                make(chan interface{}),
		addCh:                 make(chan interface{}),
		handler:               handler,
		pendingNotifications:  *buffer.NewRingGrowing(bufferSize),
		requestedResyncPeriod: requestedResyncPeriod,
		resyncPeriod:          resyncPeriod,
	}

	ret.determineNextResync(now)

	return ret
}

func (p *processorListener) add(notification interface{}) {
	p.addCh <- notification
}

func (p *processorListener) pop() {
	defer utilruntime.HandleCrash()
	defer close(p.nextCh) // Tell .run() to stop

	var nextCh chan<- interface{}
	var notification interface{}
	for {
		select {
		case nextCh <- notification:
			// Notification dispatched
			var ok bool
			notification, ok = p.pendingNotifications.ReadOne()
			if !ok { // Nothing to pop
				nextCh = nil // Disable this select case
			}
		case notificationToAdd, ok := <-p.addCh:
			if !ok {
				return
			}
			if notification == nil { // No notification to pop (and pendingNotifications is empty)
				// Optimize the case - skip adding to pendingNotifications
				notification = notificationToAdd
				nextCh = p.nextCh
			} else { // There is already a notification waiting to be dispatched
				p.pendingNotifications.WriteOne(notificationToAdd)
			}
		}
	}
}

func (p *processorListener) run() {
	// this call blocks until the channel is closed.  When a panic happens during the notification
	// we will catch it, **the offending item will be skipped!**, and after a short delay (one second)
	// the next notification will be attempted.  This is usually better than the alternative of never
	// delivering again.
	stopCh := make(chan struct{})
	wait.Until(func() {
		for next := range p.nextCh {
			switch notification := next.(type) {
			case updateNotification:
				p.handler.OnUpdate(notification.oldObj, notification.newObj)
			case addNotification:
				p.handler.OnAdd(notification.newObj)
			case deleteNotification:
				p.handler.OnDelete(notification.oldObj)
			default:
				utilruntime.HandleError(fmt.Errorf("unrecognized notification: %T", next))
			}
		}
		// the only way to get here is if the p.nextCh is empty and closed
		close(stopCh)
	}, 1*time.Second, stopCh)
}

// shouldResync deterimines if the listener needs a resync. If the listener's resyncPeriod is 0,
// this always returns false.
func (p *processorListener) shouldResync(now time.Time) bool {
	p.resyncLock.Lock()
	defer p.resyncLock.Unlock()

	if p.resyncPeriod == 0 {
		return false
	}

	return now.After(p.nextResync) || now.Equal(p.nextResync)
}

func (p *processorListener) determineNextResync(now time.Time) {
	p.resyncLock.Lock()
	defer p.resyncLock.Unlock()

	p.nextResync = now.Add(p.resyncPeriod)
}

func (p *processorListener) setResyncPeriod(resyncPeriod time.Duration) {
	p.resyncLock.Lock()
	defer p.resyncLock.Unlock()

	p.resyncPeriod = resyncPeriod
}
```