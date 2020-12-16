# wait.BackoffUntil 示例与源码分析

## 示例
```go
package main

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
	"log"
	"time"
)

func init() {
	log.SetFlags(log.Lmicroseconds)
}

// main 中使用 wait.BackoffUntil 在1分钟内每隔 10s 打印一次 "Doing task..."
func main() {
	// stopCh 通知 wait.BackoffUntil 停止执行
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

	// wait.BackoffUntil 在每个周期循环执行指定的任务直到 stopCh 发送停止信号.
	// BackoffManager 管理执行任务的周期, 目前支持两种 BackoffManager
	//   1. jitteredBackoffManagerImpl
	//   2. exponentialBackoffManagerImpl
	//
	// sliding 如果为 true, 则在任务结束时计算周期, 反之任务执行时间也计算在内
	// jitter 设置波动的范围
	wait.BackoffUntil(func() {
		log.Println("Doing task...")
	}, wait.NewJitteredBackoffManager(time.Second*10, 0.0, clock.RealClock{}), true, stopCh)
}
```

## 源码分析

BackoffUntil 在每个 BackoffManager 给与的周期运行函数, 
直到 stop channel 收到结束信号或被关闭.
sliding 如果为 true, 则在函数结束时计算周期, 反之任务执行时间也计算在内.

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

		// NOTE: b/c there is no priority selection in golang
		// it is possible for this to race, meaning we could
		// trigger t.C and stopCh, and t.C select falls through.
		// In order to mitigate we re-check stopCh at the beginning
		// of every loop to prevent extra executions of f().

        // 注意, 因为 golang 中 select 没有优先级, 每个 case 都可能执行, 
        // 这可能会导致竞争 (race), 也就是说我们可以触发 t.C 和 stopCh, t.C select 失败.
        // 为了减轻这种竞争, 在 每次循环开始是, 都校验 stopCh, 这可以避免当
        // stopCh 和 t.C() 通知就绪时, select 了t.C(), 来防止额外执行 stopCh
		select {
		case <-stopCh:
			return
		case <-t.C():
		}
	}
}
```

### BackoffManager分析
TODO
