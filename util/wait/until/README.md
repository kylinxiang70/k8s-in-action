# wait.Until 示例与源码分析

## 示例

```go
package main

import (
	"context"
	"k8s.io/apimachinery/pkg/util/wait"
	"log"
	"time"
)

func init() {
	log.SetFlags(log.Lmicroseconds)
}

// main 中使用 wait.Until 在1分钟内每隔10s 打印一次 "Doing task..."
func main() {
	// stopCh 通知 BackOffUtil 停止执行
	stopCh := make(chan struct{})

	// 创建一个带超时的 Context
	ctx, _ := context.WithTimeout(context.Background(), time.Minute)

	go func(ctx context.Context, stopCh chan struct{}) {
		for {
			select {
			case <-ctx.Done():
				stopCh <- struct{}{} // 也可以 close(stopCh)
				return
			default:
			}
		}
	}(ctx, stopCh)

	// Until 在每个周期循环执行指定的任务直到 stopCh 发送停止信号.
	// Until 是 JitterUntil 基础上的语法糖, jitter factor 为 0,
	// sliding = true(意味着duration从方法结束后开始计算)
	wait.Until(func() {
		log.Println("Doing task...")
	}, time.Second*10, stopCh)
}
```

## 源码分析

wait.Until 基于 wait.JitterUntil 实现.

wait.JitterUntil 可以用来设置每次循环时, duration 在某个范围内随机波动.
wait.Until 调用 wait.JitterUntil 将 jitterFactor 参数设为 0.0, 
相当于不设置随机波动.
sliding 设置 true, 表示 duration 从函数执行结束后开始算起.

wait.JitterUntil 详细分析见 [wait.JitterUntil 源码分析](../jitteruntil/README.md)

```go
// Until 循环执行指定的任务直到 stopCh 发送停止信号.
//
// Until 是 JitterUntil 基础上的语法糖, jitter factor 为 0,
// sliding = true(意味着duration从方法结束后开始计算)
func Until(f func(), period time.Duration, stopCh <-chan struct{}) {
	JitterUntil(f, period, 0.0, true, stopCh)
}
```
