/**
 * @author xiangqilin
 * @date 2020/12/10
**/
package main

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func main() {
	depGVR := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	fmt.Println(depGVR)
}
