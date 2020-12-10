package main

import (
	"fmt"

	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// 读取kubeconfig信息
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/xiangqilin/.kube/config")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", config)
}
