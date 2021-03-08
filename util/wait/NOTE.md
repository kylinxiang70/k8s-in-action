# 重点代码片段学习

## 传递 stopCh 要谨慎

不要在连续的包含 for-select 结构的函数中传递 stopCh. 最好将 stopCh 传递到下一个函数时进行封装.
如下面的代码所示.

代码路径: staging/src/k8s.io/apimachinery/pkg/util/wait/wait.go

```go
func PollImmediateUntil(interval time.Duration, condition ConditionFunc, stopCh <-chan struct{}) error {
	done, err := condition()
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	select {
	case <-stopCh:
		return ErrWaitTimeout
	default:
		return PollUntil(interval, condition, stopCh)
	}
}

func PollUntil(interval time.Duration, condition ConditionFunc, stopCh <-chan struct{}) error {
	ctx, cancel := contextForChannel(stopCh)
	defer cancel()
	return WaitFor(poller(interval, 0), condition, ctx.Done())
}

func WaitFor(wait WaitFunc, fn ConditionFunc, done <-chan struct{}) error {
	stopCh := make(chan struct{})
	defer close(stopCh)
	c := wait(stopCh)
	for {
		select {
		case _, open := <-c:
			ok, err := runConditionWithCrashProtection(fn)
			if err != nil {
				return err
			}
			if ok {
				return nil
			}
			if !open {
				return ErrWaitTimeout
			}
		case <-done:
			return ErrWaitTimeout
		}
	}
}

func contextForChannel(parentCh <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-parentCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}
```

如上面的代码所示, PollImmediateUntil 基于 PollUntil 实现, PollUntil 基于 WaitFor 实现, 三个函数中都有一个 stopCh 用来通知停止.
PollImmediateUntil 和 WaitFor 方法中都有 for-select 结构, 如果直接传递 stopCh 则会导致两个 for-select 结构中只有一个被通知.
所以可以参考 contextForChannel 函数, 使用 context 包装一下 stopCh.

## timer.After 的优化

```go
// 这里使用 time.After 可能更加方便, 但是有可能
// 导致 timer 存在更长的时间（如果此 goroutine 推出得早)
// 因为 time.After 使用 NewTimer(d).C 直接返回一个 channel,
// 不能调用 Stop() 停止 timer.
go func(){
    ...
    timer := time.NewTimer(timeout)
    after = timer.C
    defer timer.Stop()
    ....
}
```

## 当 channel 的接收方没有准备好, 使用 select 检查

```go
for {
	select {
	case <-tick.C:
		// 如果 consumer 没有准备好的话, 就先跳过这个信号, 校验其他的 channel.
		// 这里避免 consumer 一直未准备好, 在这里挂起.
		select {
		case ch <- struct{}{}:
			default:
		}
	case <-after:
	    return
	case <-done:
	    return
	}
}
```

## for-select 双重检查来防止额外执行

```go
func BackoffUntil(f func(), backoff BackoffManager, sliding bool, stopCh <-chan struct{}) {
	var t clock.Timer
	for {
		select {
		case <-stopCh:
			return
		default:
		}
        
		// sliding 为 false 会将 函数耗时计算在 duration 之内
		if !sliding {
			t = backoff.Backoff()
		}

		func() {
			defer runtime.HandleCrash()
			f()
		}()

        // sliding 为 true 会在函数执行完毕之后计算 duration
		if sliding {
			t = backoff.Backoff()
		}

        // 注意, 因为 golang 中 select 没有优先级, 每个 case 都可能执行, 
        // 这可能会导致竞争 (race), 也就是说我们可以触发 t.C 和 stopCh, t.C select 失败.
        // 为了减轻这种竞争, 在 每次循环开始时, 都校验 stopCh, 这可以避免当
        // stopCh 和 t.C() 通知就绪时, 如果 select 了 t.C(), 下一次循环仍可以放置额外执行一次 f()
		select {
		case <-stopCh:
			return
		case <-t.C():
		}
	}
}
```