/**
 * @author xiangqilin
 * @date 2020/12/9
**/
package main

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
)

// main 函数使用 wait.Forever 每隔 10s 打印一次 "Doing task..."
func main() {
	// 每隔1s打印经过的时间
	go func() {
		i := 1
		for {
			time.Sleep(time.Second * 1)
			fmt.Println(i)
			i = i + 1
		}
	}()

	// Forever 不断周期性地执行指定的任务
	//
	// Forever 是 wait.Until 基础上的语法糖
	wait.Forever(func() {
		fmt.Println("Doing task...")
	}, time.Second*10)
}
