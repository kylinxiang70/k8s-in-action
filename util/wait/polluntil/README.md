# PollUntil
PollUntil 用来不断周期性的判断一个状态, 该状态抽象为一个 condition 函数, 返回 true 或 false.

```go
// PollUntil 不断尝试一个 condition func 直到返回 true, 或出现错误, 或 stopCh 被关闭.

// PollUntil 在第一次运行 `condition` 前总是等待 interval 的时间间隔.
// `condition` 至少会被执行一次.

func PollUntil(interval time.Duration, condition ConditionFunc, stopCh <-chan struct{}) error {
	ctx, cancel := contextForChannel(stopCh)
	defer cancel()
	return WaitFor(poller(interval, 0), condition, ctx.Done())
}
```


```go
// contextForChannel derives a child context from a parent channel.
//
// The derived context's Done channel is closed when the returned cancel function
// is called or when the parent channel is closed, whichever happens first.
//
// Note the caller must *always* call the CancelFunc, otherwise resources may be leaked.
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