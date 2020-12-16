/**
 * @author xiangqilin
 * @date 2020/12/9
**/
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
