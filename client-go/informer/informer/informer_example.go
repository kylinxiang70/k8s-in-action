/**
 * @author xiangqilin
 * @date 2020/12/8
**/
package main

import (
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"log"
	"path"
	"time"
)

func main() {
	homedir := homedir.HomeDir()
	configPath := path.Join(homedir, ".kube/config")
	fmt.Println(configPath)
	config, err := clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		panic(err)
	}

	// 创建 ClientSet 对象, Informer 需要通过 ClientSet 和 Kubernetes API server 通信
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// 创建 stopCh 对象，该对象用于在应用程序退出之前通知 Informer 提前退出
	stopCh := make(chan struct{})
	defer close(stopCh)

	// NewSharedInformerFactory(client kubernetes.Interface, defaultResync time.Duration) SharedInformerFactory
	// 实例化 sharedInformer 对象, 它接收两个参数:
	// 1. client: 用于和 Kubernetes API server通信
	// 2. defaultResync: 设置多久时间进行一次 Resync (重新同步), Resync会周期性执行List操作，
	//       将所有资源都存放在 Informer Store 中, 如果该参数为0, 则不同步.
	sharedInformers := informers.NewSharedInformerFactory(clientSet, time.Minute)

	// 得到具体资源对象的informer对象
	informer := sharedInformers.Core().V1().Pods().Informer()

	// 通过 informer.AddEventHandler函数为资源对象添加资源对象的回调方法，支持3种资源事件的回调方法:
	// 1. AddFunc: 当创建资源时触发的事件回调方法
	// 2. UpdateFunc: 当更新资源时触发的事件回调方法
	// 3. DeleteFunc: 当删除资源时触发的事件回调用法
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// Kubernetes 使用 Informer机制时触发资源的回调方法，将资源对象推送到 WorkQueue 或其他队列，
		// 这里直接触发资源的事件。
		AddFunc: func(obj interface{}) {
			mObj := obj.(v1.Object)
			log.Printf("New Pod Added to Store: %s", mObj.GetName())
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oObj := oldObj.(v1.Object)
			nObj := newObj.(v1.Object)
			log.Printf("%s with UID %s has been updated to %s with %s",
				nObj.GetName(), nObj.GetUID(), oObj.GetName(), oObj.GetUID())
		},
		DeleteFunc: func(obj interface{}) {
			dObje := obj.(v1.Object)
			log.Printf("Resource %s has been deleted.", dObje.GetName())
		},
	})

	// 运行 informer
	informer.Run(stopCh)
}
