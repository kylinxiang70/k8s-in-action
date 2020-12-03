/**
 * @author xiangqilin
 * @date 2020/12/3
**/
package main

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamic2 "k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"log"
)

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/xiangqilin/.kube/config")
	if err != nil {
		log.Fatal(err)
	}

	dynamic, err := dynamic2.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	gvr := schema.GroupVersionResource{
		Version:  "v1",
		Resource: "pods",
	}

	unstructedObj, err := dynamic.Resource(gvr).Namespace(corev1.NamespaceDefault).List(context.TODO(), v1.ListOptions{Limit: 100})
	if err != nil {
		log.Fatal(err)
	}

	podList := &corev1.PodList{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructedObj.UnstructuredContent(), podList)
	if err != nil {
		log.Fatal(err)
	}

	for _, pod := range podList.Items {
		fmt.Println(pod.Name)
	}

}
