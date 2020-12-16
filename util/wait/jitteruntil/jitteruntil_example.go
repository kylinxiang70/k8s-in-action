/**
 * @author xiangqilin
 * @date 2020/12/9
**/
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

// main 中使用 wait.JitterUntil 在1分钟内每隔 10s 打印一次 "Doing task...
// 60s 超时退出
func main() {
	// stopCh 通知 BackOffUtil 停止执行
	stopCh := make(chan struct{})

	// 创建一个带超时的 Context
	ctx, _ := context.WithTimeout(context.Background(), time.Minute)

	// 执行任务
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
