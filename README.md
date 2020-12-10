# k8s-in-action

本项目主要以郑东旭所著的《Kubernetes源码分析》为线索,
 基于[Kubernetes](https://github.com/kubernetes/kubernetes)  v1.19.2 
 进行了实践和源码分析, 主要包括代码示例 (以 _example 结尾的 go 文件) 和源码分析 (.md文件).
 
 在此基础上, 本项目还对 [Kubernetes](https://github.com/kubernetes/kubernetes)
 中的 Go 语言设计模式和工具包进行了分析.

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
        - [RESTClient](./client-go/client/restclient)
        - [ClientSet](./client-go/client/clientset)
        - [DynamicClient](./client-go/client/discoveryclient)
        - [DiscoveryClient](./client-go/client/discoveryclient)
    - [informer](./client-go/informer)
4. etcd 存储实现
    - TODO
5. kube-apiserver
    - TODO
6. kube-scheduler
    - TODO
7. kubernetes 中的设计模式
    - TODO
8. [工具包的实例和源码分析](./util)
    -  [k8s.io/apimachinery/pkg/util/wait](./util/wait)
