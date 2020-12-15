/**
 * @author xiangqilin
 * @date 2020/12/14
**/
package main

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"log"
	"math"
	"time"
)

// 设置 request() 超时为 2s
const timeoutDuration = 2 * time.Second

// 设置日志的格式, date + time
func init() {
	log.SetFlags(log.Ldate | log.Ltime)
}

// request 用来模拟一个延迟为 latency 的请求, ctx 用来控制超时来取消执行当前 request 的 goroutine.
// 使用 finish channel 来通知主协程请求完成
func request(ctx context.Context, finish chan struct{}, latency time.Duration) {
	fmt.Printf("Request latency %v\n", latency)
	time.Sleep(latency)
	finish <- struct{}{} // 请求完成.
}

func retry() {
	// 初始的 Backoff为 2s, 最大 backoff 为 20s,
	// 设置 resetDuration 为 math.MaxInt64, 表示永远不重置重试间隔
	// factor 因子为 2.0, 即每次重试的间隔时间扩大为原来的 2 倍
	// jitter 因子为 0.0, 表示不设置波动
	// 使用真实时钟
	backoffManager := wait.NewExponentialBackoffManager(
		time.Second*2, time.Second*20, time.Duration(math.MaxInt64), 2.0, 0.0, clock.RealClock{})

	for {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
		finish := make(chan struct{})

		log.Println("Retry to send request...")
		go request(ctx, finish, time.Second*time.Duration(randomRangeInt(0, 10)))

		timer := backoffManager.Backoff()
		select {
		case <-ctx.Done():
			cancel()
		case <-finish:
			log.Println("Request finish...")
			return
		}

		select {
		case <-timer.C():
			fmt.Println(time.Now())
		}
	}
}

func randomRangeInt(min, max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max-min) + min
}

// 场景: 随着重试次数增加, 增加重试的时间间隔.
//
// 使用 request() 模拟一个随机延迟在 [0~10) 的请求,
// request() timeout 的时间为 2s,
// 若 request 超时, 则立即开始重试,
// 重试从间隔从 2s 开始, 最大为 20s, 下一次间隔为上一次间隔的 2 倍,
// 即重试时间间隔为 0 (第一次重试,立即开始), 2, 4, 8, 16, 20, 20, 20...
func main() {
	// 设置请求超时时间为 2s.
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	finish := make(chan struct{})
	// 模拟请求
	log.Println("Sending request ...")
	go request(ctx, finish, time.Second*time.Duration(randomRangeInt(0, 10)))

	select {
	case <-ctx.Done():
		log.Println("Request timeout!")
		cancel()
		retry()
	case <-finish:
		fmt.Println("Request finish...")
	}
}
