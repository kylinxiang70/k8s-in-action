# wait.Forever 示例与源码分析

## 示例

```go
package main

import (
	"k8s.io/apimachinery/pkg/util/wait"
	"log"
	"time"
)

func init() {
	log.SetFlags(log.Lmicroseconds)
}

// main 函数使用 wait.Forever 每隔 10s 打印一次 "Doing task..."
func main() {
	// Forever 不断周期性地执行指定的任务
	//
	// Forever 是 wait.Until 基础上的语法糖
	wait.Forever(func() {
		log.Println("Doing task...")
	}, time.Second*10)
}
```

## 源码分析

wait.Forever 基于 wait.Until 实现.

wait.Until 需要传递一个 stop channel 来通知 Until 函数停止执行指定的函数, 
wait.Forever 调用 wait.Until 时, 传递 wait.NeverStop 通道来实现一直持久运行
(因为无法向 wait.NeverStop 发送消息或关闭它).

wait.Until 详细分析见 [wait.Until 示例与源码分析](../until).

```go
    // Forever 不断周期性地执行指定的任务
	//
	// Forever 是 wait.Until 基础上的语法糖
    func Forever(f func(), period time.Duration) {
    	Until(f, period, NeverStop)
    }
```