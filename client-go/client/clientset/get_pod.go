/**
 * @author xiangqilin
 * @date 2020/12/3
**/
package main

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
)

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/xiangqilin/.kube/config")
	if err != nil {
		panic(err)
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	podClient := clientSet.CoreV1().Pods(corev1.NamespaceDefault)
	list, err := podClient.List(context.TODO(), metav1.ListOptions{Limit: 100})
	if err != nil {
		log.Fatal(err)
	}

	for _, pod := range list.Items {
		fmt.Println(pod.Name)
	}
}
