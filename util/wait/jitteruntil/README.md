# wait.JitterUntil 示例与源码分析

## 示例

```go
package main

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/wait"
	"log"
	"time"
)

func init() {
	log.SetFlags(log.Lmicroseconds)
}

// main 中使用 wait.JitterUntil 在1分钟内每隔 10s 打印一次 "Doing task..."
func main() {
	// stopCh 通知 BackOffUtil 停止执行
	stopCh := make(chan struct{})

	// 创建一个带超时的 Context
	ctx, _ := context.WithTimeout(context.Background(), time.Minute)

	// 开始计时，每隔一秒打印时间
	go func(ctx context.Context, stopCh chan struct{}) {

		// select 多路复用判断是否超时
		for {
			select {
			case <-ctx.Done():
				// 时间到了
				fmt.Println("timeout!")
				// 通知 BackoffUtil 停止执行
				stopCh <- struct{}{} // 也可以 close(stopCh)

				return
			default:
			}
		}
	}(ctx, stopCh)

	// JitterUntil 在每个周期循环执行指定的函数直到 stopCh 发送停止信号.
	//
	// 如果 jitterFactor 为正数, 周期会在函数运行前调整
	// 如果 jitterFactor 不是正数(负数或者零), 周期不变
	//
	// sliding 如果为 true, 则在任务结束时计算周期, 反之任务执行时间也计算在内
	//
	// 关闭 stopCh 来停止任务执行. 如果想一直执行, 可以传递 wait.NeverStop 通道.
	// wait.Forever 通过传递 wait.NeverStop 来实现.
	wait.JitterUntil(func() {
		fmt.Println("Doing task...")
	}, time.Second*10, 0.0, true, stopCh)
}
```

## 源码分析

JitterUntil 基于 BackoffUntil 实现,
JitterUntil 使用 NewJitteredBackoffManager 创建了一个 
jitteredBackoffManagerImpl 对象 (k8s内置的 BackoffManager 的实现).

jitteredBackoffManagerImpl 用来设置循环中的波动周期
(每次循环的 duration 在一定范围内随机波动).

wait.BackoffUntil 详细分析见 [wait.BackoffUntil 示例与源码分析](../backoffuntil/README.md).

```go
    // JitterUntil 在每个周期循环执行指定的任务直到 stopCh 发送停止信号.
	//
	// 如果 jitterFactor 为正数, 周期会在函数运行前调整
	// 如果 jitterFactor 不是正数(负数或者零), 周期不变
	//
	// sliding 如果为 true, 则在任务结束时计算周期, 反之任务执行时间也计算在内
	//
	// 关闭 stopCh 来停止任务执行. 如果想一直执行, 可以传递 wait.NeverStop 通道.
	// wait.Forever 通过传递 wait.NeverStop 来实现.
    func JitterUntil(f func(), period time.Duration, jitterFactor float64, sliding bool, stopCh <-chan struct{}) {
    	BackoffUntil(f, NewJitteredBackoffManager(period, jitterFactor, &clock.RealClock{}), sliding, stopCh)
    }
```