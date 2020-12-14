# k8s-in-action

本项目主要以郑东旭所著的《Kubernetes源码分析》为线索,
 基于[Kubernetes](https://github.com/kubernetes/kubernetes)  v1.19.2 
 进行了实践和源码分析, 主要包括代码示例 (以 _example 结尾的 go 文件) 和源码分析 (.md文件).
 
 在此基础上, 本项目还对 [Kubernetes](https://github.com/kubernetes/kubernetes)
 中的 Go 语言设计模式和工具包进行了分析.
 
 How and Why!!! 本项目是对《Kubernetes源码分析》的深度扩展, 从示例和源码两个调度进行阐述.

**k8s相关的源码放在了[./staging](./staging)目录下, 推荐在 IDE 中 Debug 模式下结合 example 和 .md 阅读源码.**

项目结构如下:

1. [Kubernetes 核心数据结构](./core-data-structure)
    - [核心数据结构GVR(Group/Versions/Resource)](./core-data-structure/gvr)
    - [runtime.Object详解](./core-data-structure/runtime-object)
    - [Unstructed数据](./core-data-structure/unstructed-data)
    - [Scheme资源注册表](./core-data-structure/scheme)
    - [Codec编解码器](./core-data-structure/codec)
    - [Convert资源版本转换器](./core-data-structure/convertion)

2. Kubectl
    - TODO

3. [client-go](./client-go)
    - [client客户端对象](./client-go/client)
        - [kubeconfig](./client-go/client/kubeconfig)
        - [RESTClient](./client-go/client/restclient)
        - [ClientSet](./client-go/client/clientset)
        - [DynamicClient](./client-go/client/discoveryclient)
        - [DiscoveryClient](./client-go/client/discoveryclient)
    - [informer](./client-go/informer)
    
4. [etcd 存储实现](./etcd)
    - TODO
    
5. [kube-apiserver](./apiserver)
    - TODO
    
6. [kube-scheduler](./scheduler)
    - TODO
    
7. [kubernetes 中的设计模式](./design-pattern)
    - TODO
    
8. [工具包的实例和源码分析](./util)
    -  [k8s.io/apimachinery/pkg/util/wait](./util/wait)
