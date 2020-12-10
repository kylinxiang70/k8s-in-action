# ResourceList

> Kubernetes Group/Verseions/Resource 等核心数据结构源码放在 Kubernetes 项目下的
vendor/k8s.io/apimachinery/pkg/apis/meta/v1 包中.
>
>它包含了 Kubernetes 集群中所有组件使用的通用核心数据结构, 
>例如 `APIGroup`, `APIVersions`, `APIResource` 等.

可以通过 `APIResourceList` 数据结构描述所有 Group/Versions/Resource.

以 Pod/Service/Deployment 为例:


