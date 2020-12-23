# PollImmediateUntil

PollImmediateUntil 基于 PollUntil 实现.
PollImmediateUntil 可以不断的通过 condition 函数周期性的去检测一个状态, condition 返回 true 或 false.

```go
// PollImmediateUntil 不断尝试 condition func 直到其返回 true, 或者一个错误, 或者 stopCh 被关闭.
// PollImmediateUntil 在等待 interval 时间间隔前运行 `condition` 函数 (和 `PollUntil` 正好相反).
// `condition` 函数至少会被调用一次.
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
```