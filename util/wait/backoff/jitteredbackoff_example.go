/**
 * @author xiangqilin
 * @date 2020/12/14
**/
package main

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
	"log"
	"time"
)

// 设置日志, 方便查看时间
func init() {
	log.SetFlags(log.Lmicroseconds)
}

// 场景:在随机的间隔时间内循环执行任务
//
// 设置间隔的基础时间为 2s, jitter 为 2.0, 波动的间隔为 2s ~ 2s * 2 + 2s, 即 2s ~ 6s
func main() {
	// 基础间隔时间
	const baseDuration = time.Second * 2

	// jitter 为波动系数, 最终的 duration 为 duration ~ duration + duration * jitter
	jBackoffManager := wait.NewJitteredBackoffManager(baseDuration, 2.0, &clock.RealClock{})
	counter := 0

	for {
		// 如果执行打印超过 10, 停止执行
		if counter >= 10 {
			fmt.Println("Doing more than 10¬ times.")
			break
		}

		log.Printf("NO.%d loop!", counter)
		counter++

		// 每次执行完打印, 使用 Backoff() 方法重新获取一个新的波动的 timer.
		timer := jBackoffManager.Backoff()
		t := timer.C()

		select {
		// 等待时间到
		case <-t:
		}
	}
}
