# wait

k8s.io/apimachinery/pkg/util/wait/wait.go 提供了执行周期性任务的API.

这些 API 主要提供以下通途:

- 永久性间隔一定周期执行指定任务. 比如, 永久每隔 10s 循环执行一个指定任务.
    - 相关 API: wait.Forever()
    - [wait.Forever() 示例及源码分析](./forever)
- 可退出的、间隔一定周期执行指定任务. 比如, 每隔 10s 执行一个指定任务, 1 分钟后退出.
    - 相关 API: wait.Util()
    - [wait.Until() 示例及源码分析](./until)
- 可退出的、可指定间隔时间在一定范围内波动的周期性任务. 比如, 每次间隔随机 1~5s 执行周期性任务, 1 分钟后退出.
    - 相关 API: wait.JitterUntil()
    - [wait.JitterUntil() 示例及源码分析](./jitteruntil)
- 可退出的、可指定时间间隔是否包含任务执行时间的、可指定间隔时间在一定范围内波动的周期性任务. 
比如, 每次间隔随机 1~5s 执行周期性任务, 时间间隔的计算可以(不)包含任务执行时间, 1 分钟后退出.
    - 相关 API: wait.BackoffUtil()
    - [wait.BackoffUntil() 示例及源码分析](./backoffuntil), BackoffUntil() 支持传入一个 BackoffManger 来控制循环周期.
 
以上 API 的实现关系如下所示(下层实现上层): 
```
    wait.Forever()
    wait.Until()
    wait.JitterUntil()
    wait.BackoffUtil()
```

除此之外, wait.BackoffUtil() 可以指定传入一个 BackoffManager 参数来实现周期控制.
BackoffManager 是一个接口, 声明了唯一的方法 Backoff() 来返回一个 timer.

k8s 目前对 BackoffManager 有两种实现:
- exponentialBackoffManagerImpl: 实现了指数增长的时间间隔, 并支持抖动
    - 应用场景: 比如针对微服务请求, 上游服务不健康时, 需要执行重试操作, 重试间隔时间指数增长, 来避免频繁重试, 带来的资源的过多消耗.
- jitteredBackoffManagerImpl: 实现了在一定范围内抖动的时间间隔
    - 应用场景: 比如游戏场景中, 随机事件间隔刷怪.    
  
[exponentialBackoffManagerImpl 和 jitteredBackoffManagerImpl 示例及源码详解](./backoff)