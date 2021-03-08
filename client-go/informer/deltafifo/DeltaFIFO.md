# DeltaFIFO 源码分析

## 创建 DeltaFIFO

```go
## 函数
// 弃用: 等价于 NewDeltaFIFOWithOptions(DeltaFIFOOptions{KeyFunction: keyFunc, KnownObjects: knownObjects})
func NewDeltaFIFO(keyFunc KeyFunc, knownObjects KeyListerGetter) *DeltaFIFO {
	return NewDeltaFIFOWithOptions(DeltaFIFOOptions{
		KeyFunction:  keyFunc,
		KnownObjects: knownObjects,
	})
}

// DeltaFIFOOptions 是构建 DeltaFIFO 的可选配置参数.
type DeltaFIFOOptions struct {
	// KeyFunction 用来计算一个对象对应的 key (通过 DeltaFIFO 的 KeyOf() 方法暴露,
	// 并对已删除对象和队列状态进行额外处理)
	// 默认的方法是 MetaNamespaceKeyFunc, 源码位置:k8s.io/client-go/tools/cache/store.go
	KeyFunction KeyFunc

	// KnownObjects将返回该队列的使用者“知道”的键列表(主要指 indexer). 当Replace()方法(全量更新)被调用时，它用来决定哪些 item 缺失了;
	// 这些缺失的 item 将会生成对应的 Deleted Delta. 如果你能容忍调用 Replace() 时导致删除操作缺失,
	// 那么 KnownObjects 可能是nil。
	KnownObjects KeyListerGetter

	// EmitDeltaTypeReplaced 表示队列使用者知道 Replaced DeltaType.
	// 在添加 Replaced DeltaType 之前, Replace()调用的处理方式与Sync()相同.
	// 为了向后兼容, 默认为false. 
	// 当为true时，调用 Replace() 方法设置的 DeltaType 为 Replaced; 
	// 当为false时，将设置 `Sync` DeltaType.
	EmitDeltaTypeReplaced bool
}

// NewDeltaFIFOWithOptions 返回一个 Queue, 可以用来处理 items 的变化.
func NewDeltaFIFOWithOptions(opts DeltaFIFOOptions) *DeltaFIFO {
	if opts.KeyFunction == nil {
		opts.KeyFunction = MetaNamespaceKeyFunc
	}

	f := &DeltaFIFO{
		items:        map[string]Deltas{},
		queue:        []string{},
		keyFunc:      opts.KeyFunction,
		knownObjects: opts.KnownObjects,

		emitDeltaTypeReplaced: opts.EmitDeltaTypeReplaced,
	}
	f.cond.L = &f.lock
	return f
}
```

## DeltaFIFO 结构

DeltaFIFO 是 Reflector 到 Indexer 之间的桥梁.

DeltaFIFO 类似于 FIFO, 但是有两点不同.
1. DeltaFIFO中存储了 Kubernetes 中 对象状态的变化 Delta (Added, Updated, Deleted, Synced),
DeltaFIFO 维护了一个 map, key 为对象生成的键, 值为 Deltas, Deltas 是 Delta对象的切片, 
因为对一个状态的操作会产生很多状态, 因此形成了一个状态序列.
DeltaFIFO 中的对象可以被删除, 删除状态用 DeletedFinalStateUnknown 对象表示.

2. 另一个区别是 DeltaFIFO 有一种额外的方法 Sync, Sync 是同步全量数据到 Indexer.

DeltaFIFO 是一个生产者消费者队列, Reflector 是生产者, 调用 Pop() 方法的为消费者(Indexer).

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
### Delta 和 Deltas
```go
// Delta 是 DeltaFIFO 存储的类型. 它可以告诉你发生了什么变化, 和变化之后对象的状态.
// 除非变化是删除, 然后对象被删除前, 你将会得到对象的最终状态.
type Delta struct {
	Type   DeltaType
	Object interface{}
}

// DeltaType 是变化的类型.
type DeltaType string

// 变化类型(Change type)定义
const (
	Added   DeltaType = "Added"
	Updated DeltaType = "Updated"
	Deleted DeltaType = "Deleted"
    // 当遇到 watch 错误并且必须 relist 时发生. 我们不知道 replaced object 是否变化.
    // 注意: DeltaFIFO 以前的版本需要对 Replace 事件使用 Sync. 所以, Replace 只在
    // EmitDeltaTypeReplaced 选项为 true 时发生.
	Replaced DeltaType = "Replaced"
	// Sync is for synthetic events during a periodic resync.
    // Sync 是针对周期性重新同步期间的综合事件(synthetic)
	Sync DeltaType = "Sync"
)

// Deltas 是一个或多个 `Delta` 的列表对象.
// 最老的 Delta 位于 index 0, 最新的 Delta 是列表中的最后一个.
type Deltas []Delta

// Oldest 返回最老的 Delta, 或者返回 nil 表示没有 Delta.
func (d Deltas) Oldest() *Delta {
	if len(d) > 0 {
		return &d[0]
	}
	return nil
}

// Newest 返回最新的 Delta, 或者返回 nil 表示没有 Delta.
func (d Deltas) Newest() *Delta {
	if n := len(d); n > 0 {
		return &d[n-1]
	}
	return nil
}

// copyDeltas 返回一个 d 的浅拷贝, 它只拷贝了 slice, 但是没有拷贝 slice 中的 object.
// 这允许 Get/List 返回一个我们知道不会被后续修改破坏的对象.
func copyDeltas(d Deltas) Deltas {
	d2 := make(Deltas, len(d))
	copy(d2, d)
	return d2
}

// DeletedFinalStateUnknown 将在以下情况被加入 DeltaFIFO:
// 一个 object 被删除, 但是当从 apiserver 断开连接,  watch 删除事件丢失. 这种情况下我们不知道最终
// object 状态, 所以这里有可能 `obj` 是过时的.
type DeletedFinalStateUnknown struct {
	Key string
	Obj interface{}
}
```

### KeyFunc

```go
// k8s.io/client-go/tools/cache/store.go
// KeyFunc 构建一个 object 的 key. 其实现必须具有确定性(deterministic).
type KeyFunc func(obj interface{}) (string, error)
```

### KeyListerGetter

```go
// KeyListerGetter 知道怎么 list keys 和通过 key 查找.
type KeyListerGetter interface {
	KeyLister
	KeyGetter
}

// KeyLister 知道怎么 list keys.
type KeyLister interface {
	ListKeys() []string
}

// KeyGetter 知道怎么通过给定的 key 来获取存储的值
type KeyGetter interface {
	// GetByKey returns the value associated with the key, or sets exists=false.
	GetByKey(key string) (value interface{}, exists bool, err error)
}
```

## DeltaFIFO 方法

### Close

```go
// 关闭队列
func (f *DeltaFIFO) Close() {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.closed = true
    // 通知所有的 cond, 释放锁
	f.cond.Broadcast()
}
```

### KeyOf

```go
// KeyOf 暴露了 f 对象的 keyFunc, 也用来检测 Delta 对象或者 DeletedFinalStateUnknown  对象的 key.
func (f *DeltaFIFO) KeyOf(obj interface{}) (string, error) {
	if d, ok := obj.(Deltas); ok {
		if len(d) == 0 {
			return "", KeyError{obj, ErrZeroLengthDeltasObject}
		}
        // 如果 obj 是 Deltas 类型, 则返回 deltas 中的最新的对象
		obj = d.Newest().Object
	}
	// 如果是 DeletedFinalStateUnknown 说明对象已经被删了.
	if d, ok := obj.(DeletedFinalStateUnknown); ok {
        // 如果 obj 是 DeletedFinalStateUnknown 类型, 则直接返回 d.Key
		return d.Key, nil
	}
	return f.keyFunc(obj)
}
```

### HasSynced

```go
// HasSynced 在 1或2 的情况下返回 true:
// 1. 如果 Add/Update/Delete/AddIfNotPresent 首先被调用.
// 2. 被 Replace() 插入的第一批 items 被 popped.
func (f *DeltaFIFO) HasSynced() bool {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.populated && f.initialPopulationCount == 0
}
```

### Add/Update/Delete



```go
// Add 插入一个 item, 并且将其入队. 只有当 item 在 items 中不存在时, 才会被入队.
func (f *DeltaFIFO) Add(obj interface{}) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.populated = true
    // 使用 queueActionLocked 构造一个 DeltaType 为 Added 的 Delta
	return f.queueActionLocked(Added, obj)
}
```

```go
// Update 与 Add 类似, 但是构造的是一个 Update Delta
func (f *DeltaFIFO) Update(obj interface{}) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.populated = true 
    // 使用 queueActionLocked 构造一个 DeltaType 为 updated 的 Delta
	return f.queueActionLocked(Updated, obj)
}
```

```go
// Delete 与 Add 类似, 构造一个 Deleted Delta. 如果给定的 object 不存在,
// 则被忽略 (比如: 这个对象可能被 Replace (re-list) 删掉了). 
// 本方法中, `f.knownObjects` 如果不为 nil 的话, 就提供 (通过 GetByKey)
// 被认为已经存在的 _additional_ objects.
func (f *DeltaFIFO) Delete(obj interface{}) error {
	id, err := f.KeyOf(obj)
	if err != nil {
		return KeyError{obj, err}
	}
	f.lock.Lock()
	defer f.lock.Unlock()
	f.populated = true
	// knownObjects 这里就是 Indexer, 包含了所有已知的 Key.
	if f.knownObjects == nil {
		// 没有 indexer, 就检查自已存储的对象
		if _, exists := f.items[id]; !exists {
			// 可以假设, relist 发生时, 该 item 已经被删除了.
			return nil
		}
	} else {
        // 如果该对象不存在于 knownObjects 和 items 中, 则跳过构建 deleted Delta.
        // 注意: 如果该对象已经在 items 有了一个 deleted Delta, 可以忽略它, 因为在
        //  queueActionLocked 中会自动去重.
		_, exists, err := f.knownObjects.GetByKey(id) // 检查在 Indexer 中是否存在
		_, itemsExist := f.items[id] // 检查 items 中是否存在
		// 既不在 Indexer 中, 也不在 items 中, 说明在 Replace() 的时候, 已经删除.
		if err == nil && !exists && !itemsExist {
			return nil
		}
	}

    // 构建一个 Deleted Delta, 该对象存在于 items 或/和 KnownObjects
	return f.queueActionLocked(Deleted, obj)
}
```

#### queueActionLocked

```go
// queueActionLocked 向 Deltas 添加给定对象的 Delta.
// 调用者必须加锁.
func (f *DeltaFIFO) queueActionLocked(actionType DeltaType, obj interface{}) error {
    // 使用 KeyOf 方法得到该对象的 key.
	id, err := f.KeyOf(obj)
	if err != nil {
		return KeyError{obj, err}
	}
    // 向该对象的 Deltas 添加 Delta.
	newDeltas := append(f.items[id], Delta{actionType, obj})
	// 使用 dedupDeltas 将 Deltas 最新的两个 Delta 进行去重(如果 Deltas[n-2] == Deltas[n-1])
	// 这里去重主要是指删除事件, 有两种情况会出现对于同一个对象有两个删除事件:
	// 1. 当 f.knownObjects 为空时, apiserver 删除了某个对象, 并且 watch 机制 watch 到了该事件(还没有Pop),
	//     但是此时触发了 Replace (全量更新), apiserver 与 items 中的数据不一致, Replace 对该对象进行如下操作:
	//     f.queueActionLocked(Deleted, DeletedFinalStateUnknown{k, deletedObj}), 将会导致出现连续的 Deleted 事件.
	// 2. 当 f.knownObjects 不会空时, apiserver 删除了某个对象, 但是该事件还未被应用到 indexer, 此时触发了 Replace,
	//     apiserver 与 items 中的数据不一致, Replace 对该对象进行如下操作:
	//     f.queueActionLocked(Deleted, DeletedFinalStateUnknown{k, deletedObj}), 将可能会导致出现连续的 Deleted 事件.
	newDeltas = dedupDeltas(newDeltas)

	if len(newDeltas) > 0 {
        // 判断 `items` 是否存在 key 为 id 的记录, 如果不存在, 则入队.
		if _, exists := f.items[id]; !exists {
			f.queue = append(f.queue, id)
		}
        // 将 items 中 key 为 id 的值更新为 newDeltas
		f.items[id] = newDeltas
        // 唤醒其他在 condition 上 wait 的 goroutine
		f.cond.Broadcast()
	} else {
        // 这里永远不会发生, 因为 newDeltas 是 调用 dedupDeltas() 得到的, 如果给 dedupDeltas()
        // 的参数不是一个空的 list, dedupDeltas() 永远不会返回空的 list.
        // 但是 如果它确实返回了一个空 list, 那么就需要将 id 对应的 item 从 items 中删除 (如果 item 不在 map 中,
		// 那么队列中的 item 也会被忽略).
		delete(f.items, id)
	}
	return nil
}

// re-listing 和 watch 可能导致相同的更新以任意顺序被执行多次. 此方法会将 Deltas 中
// 最新的两个 Delta 去重, 如果他们相同的话.
func dedupDeltas(deltas Deltas) Deltas {
	n := len(deltas)
	if n < 2 {
		return deltas
	}
	a := &deltas[n-1]
	b := &deltas[n-2]
	if out := isDup(a, b); out != nil {
        // `a` 和 `b` 是重复的, 只保留 isDup() 方法返回的那一个.
        // TODO: 这里额外分配了 array 的内存, 看起来是不必要的, 如果我们可以比较 `items`
        // 中的最后一个元素和新的 Delta, 这样就可以通过直接修改 `items` 完成.
        // 可能值得探究如果可以安全的优化.
		d := append(Deltas{}, deltas[:n-2]...)
		return append(d, *out)
	}
	return deltas
}

// 如果 a 和 b 代表相同的事件, 返回需要保留的 Delta. 否则返回 nil.
// TODO： 除了删除事件, 是否还有其他需要去重?
func isDup(a, b *Delta) *Delta {
	if out := isDeletionDup(a, b); out != nil {
		return out
	}
    // TODO: 其他需要去重的情况? 有吗？
	return nil
}

// 如果都是删除事件, 则保留信息最多那一个.
func isDeletionDup(a, b *Delta) *Delta {
	if b.Type != Deleted || a.Type != Deleted {
		return nil
	}
	// Do more sophisticated checks, or is this sufficient?
	if _, ok := b.Object.(DeletedFinalStateUnknown); ok {
		return a
	}
	return b
}
```

### AddIfNotPresent

```go
// AddIfNotPresent 直接插入一个 item, 并且将其 入队. 如果 item 已经存在于现有的 items 中,
// 则既不入队也不加入 items.
//
// 这在单个生产者/消费者场景中非常有用，这样消费者就可以安全地重试 item，而不必与生产者竞争，也不必潜在地将过期 item 放入队列。
//
// 重要: `obj` 必须是 Deltas 类型 ( Pop() 方法的输出). 这和 Add/Update/Delete 方法不同. 
func (f *DeltaFIFO) AddIfNotPresent(obj interface{}) error {
	deltas, ok := obj.(Deltas)
	if !ok {
		return fmt.Errorf("object must be of type deltas, but got: %#v", obj)
	}
	id, err := f.KeyOf(deltas.Newest().Object)
	if err != nil {
		return KeyError{obj, err}
	}
	f.lock.Lock()
	defer f.lock.Unlock()
	f.addIfNotPresent(id, deltas)
	return nil
}

// addIfNotPresent 当 items 中不存在 key 为 id 的 item 时, 向 deltas 插入, 并且假设
// 调用者已经获取了 fifo 锁.
func (f *DeltaFIFO) addIfNotPresent(id string, deltas Deltas) {
	f.populated = true
	if _, exists := f.items[id]; exists {
		return
	}

	f.queue = append(f.queue, id)
	f.items[id] = deltas
	f.cond.Broadcast()
}
```

### List

```go
// List 返回 items 中所有最新的 Delta 的 object 的列表.
// 应该将 deltas 内返回的项视为不可变的.
func (f *DeltaFIFO) List() []interface{} {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.listLocked()
}

func (f *DeltaFIFO) listLocked() []interface{} {
	list := make([]interface{}, 0, len(f.items))
	for _, item := range f.items {
		list = append(list, item.Newest().Object)
	}
	return list
}
```

### ListKeys

```go
// 返回所有当前 FIFO 中所有 object 的 key.
func (f *DeltaFIFO) ListKeys() []string {
	f.lock.RLock()
	defer f.lock.RUnlock()
	list := make([]string, 0, len(f.items))
	for key := range f.items {
		list = append(list, key)
	}
	return list
}
```

### Get/GetByKey

`Get()` 通过调用 `GetByKey()` 返回给定 `obj` 的 `deltas` 列表, 或将 `exists` 设置为 `false`.

```go
// Get 返回给定 obj 的 deltas 列表, 或将 exists 设置为 false.
// 应该将 deltas 内返回的项视为不可变的.
func (f *DeltaFIFO) Get(obj interface{}) (item interface{}, exists bool, err error) {
	key, err := f.KeyOf(obj)
	if err != nil {
		return nil, false, KeyError{obj, err}
	}
	return f.GetByKey(key)
}

// GetByKey 返回给定 key 对应的 object 的 deltas 列表, 或将 exists 设置为 false.
// 应该将 deltas 内返回的项视为不可变的.
func (f *DeltaFIFO) GetByKey(key string) (item interface{}, exists bool, err error) {
	f.lock.RLock()
	defer f.lock.RUnlock()
	d, exists := f.items[key]
	if exists {
        // 复制 item 的切片，这样对切片的操作就不会干扰我们返回的对象.
		d = copyDeltas(d)
	}
	return d, exists, nil
}
```
### Pop


```go
// Pop 方法会一直阻塞直到 item 被加入到 queue, 然后返回这个 item.
// 如果多个 item 同时 可以被 Pop, 他们会以 add/update 的顺序被 Pop.
// item 会在被 Pop 前从 quque 和 store 中被删除, 如果你没有处理成功,
// 则需要重新使用 AddIfNotPresent() 添加该 item.
// process 方法是在加锁条件下被调用的, 所以可以在 process 方法进行需要和 queue 同步的修改操作.
// PopProcessFunc 可能返回一个 ErrRequeue 对象, 暗示当前的 item 需要被 requeue (
// 这和调用 AddIfNotPresent() 方法重新添加该 item 是一个意思).
// process 方法应该避免开销大的操作, 避免其他 queue 的操作 (i.e. Add() and Get() 阻塞太久).
//
// Pop返回一个'Deltas'，它包含了对象在队列中发生的所有事情的完整列表.
func (f *DeltaFIFO) Pop(process PopProcessFunc) (interface{}, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	for {
		for len(f.queue) == 0 {
			// 当队列为空的时候, Pop() 方法将会阻塞直到有 item 入队.
			// 当 Close() 方法被调用, f.closed 将会被设置为 true, 所有在 condition 上阻塞的 goroutine 将会被广播 (breadcast) 唤醒.
			if f.closed {
				return nil, ErrFIFOClosed
			}

			f.cond.Wait()
		}
		id := f.queue[0] // 从 queue 获取第一个 id (先进先出, 从下标 0 开始)
		f.queue = f.queue[1:]
		// 需要同步对象的数目减 1, 当 initialPopulationCount 减为 0 时, 说明全部对象都同步完毕.
		if f.initialPopulationCount > 0 {
			f.initialPopulationCount--
		}
		item, ok := f.items[id]
		if !ok {
			// item 已经被删除, 则继续弹出下一个
			continue
		}
		delete(f.items, id)
		err := process(item)
		if e, ok := err.(ErrRequeue); ok {
			f.addIfNotPresent(id, item)
			err = e.Err
		}
		// Don't need to copyDeltas here, because we're transferring
		// ownership to the caller.
		return item, err
	}
}
```

### Replace

```go
// Replace 方法做了两件事:
// (1) 使用 Sync 或者 Replace DeltaType 添加给定的对象;
// (2) 然后进行一些删除操作.
// 特殊的: 对于每一个先前存在的 key K, 如果不是 list 中 objects 所对应的 key,
// 那么就将产生和 Delete(DeletedFinalStateUnknown{K, O}) 一样的效果,
// O 是当前 k 对应的对象.
// 如果 `f.knownObjects == nil`, 那么预先存在的 keys 就是 `f.items` 的 keys
// 并且 K 对应的 object 是 Deltas `.Newest()` 方法返回的最新对象.
// 然而, 先前存在的 keys 是被 `f.knownObjects ` 列举出的, 并且
// K 对应的当前对象是 `f.knownObjects.GetByKey(K)` 返回的.
func (f *DeltaFIFO) Replace(list []interface{}, resourceVersion string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	keys := make(sets.String, len(list))

	// 向后兼容
	action := Sync
	if f.emitDeltaTypeReplaced {
		action = Replaced
	}

	// 对每一个新的 item 添加 Sync/Replaced DeltaType
	for _, item := range list {
		key, err := f.KeyOf(item)
		if err != nil {
			return KeyError{item, err}
		}
		keys.Insert(key)
		if err := f.queueActionLocked(action, item); err != nil {
			return fmt.Errorf("couldn't enqueue object: %v", err)
		}
	}

	// 对比 list 中的对象和 items 中的对象,
	// 如果 item 中的对象不存在于 list 中, 则删除.
	if f.knownObjects == nil {
		queuedDeletions := 0
		for k, oldItem := range f.items {
			if keys.Has(k) {
				continue
			}
			// 删除先前存在于 items 中却不存在于 list 中的对象.
			// 这可能发生在以下情况:
			// 因为和 apiserver断开链接, 导致 watch 删除事件丢失.
			var deletedObj interface{}
			if n := oldItem.Newest(); n != nil {
				deletedObj = n.Object
			}
			queuedDeletions++
			if err := f.queueActionLocked(Deleted, DeletedFinalStateUnknown{k, deletedObj}); err != nil {
				return err
			}
		}

		// 如果 items 还没有被填充过, 则需要设置 populated 和 initialPopulationCount.
		if !f.populated {
			f.populated = true
			// 因为上面进行了 f.queueActionLocked(Deleted, DeletedFinalStateUnknown{k, deletedObj}) 操作,
			// 所以需要加上 queuedDeletions.
			f.initialPopulationCount = len(list) + queuedDeletions
		}

		return nil
	}

	// 对比 list 中的对象和 knownObjects 中的对象,
	// 如果 knownObjects 中的对象不存在于 list 中, 则删除.
	knownKeys := f.knownObjects.ListKeys()
	queuedDeletions := 0
	for _, k := range knownKeys {
		if keys.Has(k) {
			continue
		}

		deletedObj, exists, err := f.knownObjects.GetByKey(k)
		if err != nil {
			deletedObj = nil
			klog.Errorf("Unexpected error %v during lookup of key %v, placing DeleteFinalStateUnknown marker without object", err, k)
		} else if !exists {
			deletedObj = nil
			klog.Infof("Key %v does not exist in known objects store, placing DeleteFinalStateUnknown marker without object", k)
		}
		queuedDeletions++
		if err := f.queueActionLocked(Deleted, DeletedFinalStateUnknown{k, deletedObj}); err != nil {
			return err
		}
	}
    // 如果 items 还没有被填充过, 则需要设置 populated 和 initialPopulationCount.
	if !f.populated {
		f.populated = true
		// 因为上面进行了 f.queueActionLocked(Deleted, DeletedFinalStateUnknown{k, deletedObj}) 操作,
	    // 所以需要加上 queuedDeletions.
		f.initialPopulationCount = len(list) + queuedDeletions
	}

	return nil
}
```

### Resync

为什么需要 Resync 机制呢？因为在处理 SharedInformer 事件回调时，可能存在处理失败的情况，定时的 Resync 让这些处理失败的事件有了重新 onUpdate 处理的机会。

```go
// Resync 将为`f.knownObjects`列出的所有 key 添加 Sync 类型的 Delta, 
// 这些 key 正在排队等待处理.
// 如果 `f.knownObjects` 为 nil, 则 Resync 不做任何事情.
func (f *DeltaFIFO) Resync() error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.knownObjects == nil {
		return nil
	}

	// 获取 indexer 中所有的 key, 并通过 syncKeyLocked 并添加 Sync 类型的 Delta.
	keys := f.knownObjects.ListKeys()
	for _, k := range keys {
		if err := f.syncKeyLocked(k); err != nil {
			return err
		}
	}
	return nil
}

func (f *DeltaFIFO) syncKeyLocked(key string) error {
	obj, exists, err := f.knownObjects.GetByKey(key)
	if err != nil {
		klog.Errorf("Unexpected error %v during lookup of key %v, unable to queue object for sync", err, key)
		return nil
	} else if !exists {
		klog.Infof("Key %v does not exist in known objects store, unable to queue object for sync", key)
		return nil
	}

	id, err := f.KeyOf(obj)
	if err != nil {
		return KeyError{obj, err}
	}
	// 如果对应 key 的 deltas 的长度大于 0, 跳过.
	if len(f.items[id]) > 0 {
		return nil
	}

	if err := f.queueActionLocked(Sync, obj); err != nil {
		return fmt.Errorf("couldn't queue object: %v", err)
	}
	return nil
}
```

那么经过 Resync 重新放入 Delta FIFO 队列的事件，和直接从 apiserver 中 watch 得到的事件处理起来有什么不一样呢？

```go
// k8s.io/client-go/tools/cache/shared_informer.go
func (s *sharedIndexInformer) HandleDeltas(obj interface{}) error {
	s.blockDeltas.Lock()
	defer s.blockDeltas.Unlock()

	// from oldest to newest
	for _, d := range obj.(Deltas) {
		// 判断事件类型，看事件是通过新增、更新、替换、删除还是 Resync 重新同步产生的
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
					// 如果是通过 Resync 重新同步得到的事件则做个标记
					isSync = true
				case d.Type == Replaced:
					...
				}
				// 如果是通过 Resync 重新同步得到的事件，则触发 onUpdate 回调
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
从上面对 Delta FIFO 的队列处理源码可看出，如果是从 Resync 重新同步到 Delta FIFO 队列的事件，会分发到 updateNotification 中触发 onUpdate 的回调

Resync 机制的引入，定时将 Indexer 缓存事件重新同步到 Delta FIFO 队列中，在处理 SharedInformer 事件回调时，让处理失败的事件得到重新处理。
并且通过入队前判断 FIFO 队列中是否已经有了更新版本的 event，来决定是否丢弃 Indexer 缓存不进行 Resync 入队。在处理 Delta FIFO 队列中的 Resync 的事件数据时，触发 onUpdate 回调来让事件重新处理。

