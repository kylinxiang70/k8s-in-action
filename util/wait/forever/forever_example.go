/**
 * @author xiangqilin
 * @date 2020/12/9
**/
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

