/**
 * @author xiangqilin
 * @date 2020/12/9
**/
package main

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
)

// main 中使用 wait.BackoffUntil 在1分钟内每隔 10s 打印一次 "Doing task..."
func main() {
	// stopCh 通知 wait.BackoffUntil 停止执行
	stopCh := make(chan struct{})
	// stopCountCh 通知打印时间的 goroutine 停止打印
	stopCountCh := make(chan struct{})

	// 创建一个带超时的 Context
	ctx, _ := context.WithTimeout(context.Background(), time.Minute)

	// 开始计时，每隔一秒打印时间
	go func(ctx context.Context, stopCh, stopCountCh chan struct{}) {
		// 另起一个 goroutine 每秒打印当前经过的时间
		go func(chan struct{}) {
			i := 1
			for {
				select {
				case <-stopCountCh:
					return
				default:
				}
				time.Sleep(time.Second)
				fmt.Println(i)
				i = i + 1
			}
		}(stopCountCh)

		// select 多路复用判断是否超时
		for {
			select {
			case <-ctx.Done():
				// 时间到了
				fmt.Println("timeout!")
				// 通知 BackoffUtil 停止执行
				stopCh <- struct{}{} // 也可以 close(stopCh)
				// 通知另外一个 goroutine 停止打印时间
				stopCountCh <- struct{}{}
				return
			default:
			}
		}
	}(ctx, stopCh, stopCountCh)

	// wait.BackoffUntil 在每个周期循环执行指定的任务直到 stopCh 发送停止信号.
	// BackoffManager 管理执行任务的周期, 目前支持两种 BackoffManager
	//   1. jitteredBackoffManagerImpl
	//   2. exponentialBackoffManagerImpl
	//
	// sliding 如果为 true, 则在任务结束时计算周期, 反之任务执行时间也计算在内
	// jitter 设置波动的范围
	wait.BackoffUntil(func() {
		fmt.Println("Doing task...")
	}, wait.NewJitteredBackoffManager(time.Second*10, 0.0, clock.RealClock{}), true, stopCh)
}
