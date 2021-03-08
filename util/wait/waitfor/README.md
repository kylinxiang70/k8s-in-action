# wait.WaitFor 源码解析

## WaitFor 源码
```go
// WaitFor 通过 `wait` 驱动持续地检验 `fn`
// WaitFor 从 `wait()` 中得到一个 channel, 每当有一个值被传入该 channel,
// `fn` 就会被执行一次, 当 channel  被关闭时, 又会被调用一次.
// 如果 channel 被关闭并且 `fn` 未发生错误返回 false, WaitFor 返回 ErrWaitTimeout.
//
// 如果 `fn` 返回一个错误, 则 for 循环停止, 并返回该错误.
// 如果 `fn` 返回 true, 则 for 循环停止, 并返回 nil.
//
// 如果 done channel 被关闭并且 `fn` 不曾返回 true, 则返回 ErrWaitTimeout.
// 
// 当 done channel 被关闭, 因为 golang 的 `select` 语句是 "uniform pseudo-random",
// 虽然最终 `WaitFor` 将会返回, 但是 `fn` 可能仍然会运行一次或者多次.
func WaitFor(wait WaitFunc, fn ConditionFunc, done <-chan struct{}) error {
	stopCh := make(chan struct{})
	defer close(stopCh)
    // 将 stopCh 传入 waitFunc, 当 WaitFor 调用结束时, 关闭 stopCh, 
    // 同时通知 waitFunc 中的 for-select 退出.
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
```

## WaitFunc 源码

WaitFor 需传递 WaitFunc 来驱动 condition, WaitFunc 将会返回一个 channel,
WaitFor 每次从 channel 接受一个值, condition 就执行一次, 该 channel 关闭时, 
condition 执行最后一次.

```go
// WaitFunc creates a channel that receives an item every time a test
// should be executed and is closed when the last test should be invoked.
// 这个注释有点迷, 可以参见 poller 函数实现
type WaitFunc func(done <-chan struct{}) <-chan struct{}
```
wait 包中, poller 函数为 WaitFunc 类型, 可以参考实现 WaitFunc.

注释中说 "Over very short intervals you may receive no ticks 
before the channel is closed." 没看懂.
```go
// poller 函数返回一个 WaitFunc, 该 WaitFunc 每隔 interval 向 channel 发送消息 
// 直到超时, 然后关闭 channel.
// 
// Over very short intervals you may receive no ticks before the channel is closed.
// 如果 interval 非常短, 则在 channel 被关闭前可能接收不到 ticks. ???
// (
//
// timeout 如果为 0 意味着无限循环, 这种情况下, 必须由调用者关闭 done channel.
// 如果未能这样做, 就可能会造成 goroutine 泄露.
//
// 输出的 channel 是没有缓冲的. 如果 channel 没有准备好接受, 则这一次 tick 就会被跳过.
func poller(interval, timeout time.Duration) WaitFunc {
	return WaitFunc(func(done <-chan struct{}) <-chan struct{} {
		ch := make(chan struct{})

		go func() {
			defer close(ch)

			tick := time.NewTicker(interval)
			defer tick.Stop()

			var after <-chan time.Time
			if timeout != 0 {
                // 这里使用 time.After 可能更加方便, 但是有可能
                // 导致 timer 存在更长的时间（如果此 goroutine 退出得早)
                // 因为 time.After 使用 NewTimer(d).C 返回一个 channel,
				// 不能调用 Stop() 停止 timer.
				timer := time.NewTimer(timeout)
				after = timer.C
				defer timer.Stop()
			}

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
		}()

		return ch
	})
}
```

