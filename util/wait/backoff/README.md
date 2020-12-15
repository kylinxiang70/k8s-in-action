# BackoffManager 示例和源码分析

## 示例

### jitteredbackoff 示例

```go

```

### exponentialbackoff 示例

本示例展示了如何使用 exponentialbackoff 进行重试操作,
对某些不健康的上游服务, 通常失败率较高或者经常超时, 失败就要进行重试,
如果重试次数过多, 则会操作资源浪费和额外的请求压力,
这里利用 exponentialbackoff 对每次重试请求的间隔设置为指数级增长,
减轻重试压力.

```go
package main

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"log"
	"math"
	"time"
)

// 设置 request() 超时为 2s
const timeoutDuration = 2 * time.Second

// 设置日志的格式, date + time
func init() {
	log.SetFlags(log.Ldate | log.Ltime)
}

// request 用来模拟一个延迟为 latency 的请求, ctx 用来控制超时来取消执行当前 request 的 goroutine.
// 使用 finish channel 来通知主协程请求完成
func request(ctx context.Context, finish chan struct{}, latency time.Duration) {
	fmt.Printf("Request latency %v\n", latency)
	time.Sleep(latency)
	finish <- struct{}{} // 请求完成.
}

func retry() {
	// 初始的 Backoff为 2s, 最大 backoff 为 20s,
    // 设置 resetDuration 为 math.MaxInt64, 表示永远不重置重试间隔
	// factor 因子为 2.0, 即每次重试的间隔时间扩大为原来的 2 倍
	// jitter 因子为 0.0, 表示不设置波动
	// 使用真实时钟
	backoffManager := wait.NewExponentialBackoffManager(
		time.Second*2, time.Second*20, time.Duration(math.MaxInt64), 2.0, 0.0, clock.RealClock{})

	for {
		ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
		finish := make(chan struct{})

		log.Println("Retry to send request...")
		go request(ctx, finish, time.Second*time.Duration(randomRangeInt(0, 10)))

		timer := backoffManager.Backoff()
		select {
		case <-ctx.Done():
			cancel()
		case <-finish:
			log.Println("Request finish...")
			return
		}

		select {
		case <-timer.C():
			fmt.Println(time.Now())
		}
	}
}

func randomRangeInt(min, max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max-min) + min
}

// 场景: 随着重试次数增加, 增加重试的时间间隔.
//
// 使用 request() 模拟一个随机延迟在 [0~10) 的请求,
// request() timeout 的时间为 2s,
// 若 request 超时, 则立即开始重试,
// 重试从间隔从 2s 开始, 最大为 20s, 下一次间隔为上一次间隔的 2 倍,
// 即重试时间间隔为 0 (第一次重试,立即开始), 2, 4, 8, 16, 20, 20, 20...
func main() {
	// 设置请求超时时间为 2s.
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	finish := make(chan struct{})
	// 模拟请求
	log.Println("Sending request ...")
	go request(ctx, finish, time.Second*time.Duration(randomRangeInt(0, 10)))

	select {
	case <-ctx.Done():
		log.Println("Request timeout!")
		cancel()
		retry()
	case <-finish:
		fmt.Println("Request finish...")
	}
}
```

## 源码分析

BackoffManger是一个接口类型, 声明了一个方法 Backoff() 用来返回一个 Timer.

BackoffManager接口如下所示: 

代码路径: k8s.io/apimachinery/pkg/util/wait/wait.go

```go
// BackoffManager manages backoff with a particular scheme based on its underlying implementation. It provides
// an interface to return a timer for backoff, and caller shall backoff until Timer.C() drains. If the second Backoff()
// is called before the timer from the first Backoff() call finishes, the first timer will NOT be drained and result in
// undetermined behavior.
// The BackoffManager is supposed to be called in a single-threaded environment.
type BackoffManager interface {
	Backoff() clock.Timer
}
```

注释中提到: 如果在第一个 Backoff() 调用结束之前调用第二个 Backoff(), 
第一个 timer 不会被销毁, 并且具有不确定性.

**BackoffManger 应该在单线程中使用.**

---

wait 包中提供了两种 BackoffManager 的实现,
jitteredBackoffManagerImpl 和 exponentialBackoffManagerImpl. 

### jitteredBackoffManagerImpl 实现

jitteredBackoffManagerImpl 提供了一种在一定范围内波动的 duration 实现.

jitteredBackoffManagerImpl 结构体如下所示: 

```go
type jitteredBackoffManagerImpl struct {
	// clock.Clock 为接口类型, 其实现类型有 RealTime 和 FakeTime 两种.
    // RealTime 返回真实的时间, FakeTime 允许定制时间. 详情参见 k8s.io/apimachinery/pkg/util/clock/clock.go
	clock        clock.Clock
    // 时间间隔
	duration     time.Duration
    // 波动的最大系数, duraion 将在 duration ~ duration*jitter + duration 范围内随机波动
	jitter       float64
    // Backoff() 方法将返回一个新的 duration 为 duration * jitter 的 Timer.
	backoffTimer clock.Timer
}
```

jitteredBackoffManagerImpl 的 Backoff() 实现如下所示:

代码路径: k8s.io/apimachinery/pkg/util/wait/wait.go

```go
// Backoff implements BackoffManager.Backoff, it returns a timer so caller can block on the timer for jittered backoff.
// The returned timer must be drained before calling Backoff() the second time
func (j *jitteredBackoffManagerImpl) Backoff() clock.Timer {
    // Backoff() 方法通过 getNextBackoff() 来设置 backoffTimer.	
    backoff := j.getNextBackoff()

    
	if j.backoffTimer == nil {
        // 如果 backoffTimer 为空, 则新建一个 timer
		j.backoffTimer = j.clock.NewTimer(backoff)
	} else {
        // 如果 backoffTime 不为空, 则重新设置其为 backoff
		j.backoffTimer.Reset(backoff)
	}
	return j.backoffTimer
}
```

getNextBackoff() 用来控制 jitter 波动的触发条件, 
如果 jitter 参数 为正, 则会触发通过 Jitter 生成一个波动后的 duration. 
若为负数或0, 不触发波动.

getNextBackoff() 源码如下所示:

代码路径: k8s.io/apimachinery/pkg/util/wait/wait.go

```go
func (j *jitteredBackoffManagerImpl) getNextBackoff() time.Duration {
	jitteredPeriod := j.duration
	if j.jitter > 0.0 {
		jitteredPeriod = Jitter(j.duration, j.jitter)
	}
	return jitteredPeriod
}
```

Jitter 拥来时设置在 duration ~ duration * jitter + duration 范围内的波动.

jitter 源码如下所示:

代码路径: k8s.io/apimachinery/pkg/util/wait/wait.go

```go
func Jitter(duration time.Duration, maxFactor float64) time.Duration {
   // jitter 参数设置为非正和 1 的效果一样.
   if maxFactor <= 0.0 {
		maxFactor = 1.0
	}

    // 使得 duration 在 duration ~ duration * factor + duration 之间波动.
    // rand.Float64  [0,1)
    // rand.Float64*maxFactor [0,jitter)
    // rand.Float64*maxFactor*duration [0, jitter*duration)
	wait := duration + time.Duration(rand.Float64()*maxFactor*float64(duration))
	return wait
}
```

### exponentialBackoffManagerImpl 实现

exponentialBackoffManagerImpl 的结构比 jitteredBackoffManagerImpl
更加复杂, 源码如下:

代码路径:

```go
type exponentialBackoffManagerImpl struct {
    // exponentialBackoffManagerImpl 实现了 Backoff() 方法,
    // backoff 用来存放实现 Backoff() 所需的参数
	backoff              *Backoff
    // Backoff() 将会设置并返回 backoffTimer
	backoffTimer         clock.Timer
    // 上一次 Backoff() 函数开始的时间
	lastBackoffStart     time.Time
    // 初始的 backoff 间隔
	initialBackoff       time.Duration
    // backoff 重新设置的 duration,
    // Backoff() 在 backoffResetDuration 内未被调用, 将会被重新设置.
	backoffResetDuration time.Duration
    // clock.Clock 为接口类型, 其实现类型有 RealTime 和 FakeTime 两种.
    // RealTime 返回真实的时间, FakeTime 允许定制时间. 详情参见 k8s.io/apimachinery/pkg/util/clock/clock.go,
	clock                clock.Clock
}

// Backoff 用来存放 Backoff() 函数所需的参数
type Backoff struct {
	// 当前的 Duration
	Duration time.Duration
    // 如果 factor 不为 0 并且 Steps 和 Cap 没有达到限制, 
    // 那么每次迭代 Duration 都会乘以 factor.
    // Factor 不能为负.
    // jitter 不影响的 Duration 参数的修改.
	Factor float64
    // 没此迭代都会睡眠 Duration ~ Duration + Duration*Jitter 的随机时间
	Jitter float64
    // 在迭代中 Duration 参数将会修改的的剩余次数(如果 Cap 达到上限则会提前停止)
    // 如果为负, Duration 将不会改变.
    // 和 Factor、Cap 结合使用可以实现指数 backoff
	Steps int
    // 针对 Duration 参数的限制, 如果 Factor * Duration 超过了 Cap,
    // Step 参数将会被设为 0, 并且将 Duration 参数设为 Cap.
	Cap time.Duration
}
```


```go
// NewExponentialBackoffManager 返回一个管理 exponential backoff 的 manager.
/// 每个 backoff 都会波动, 并且不会超过给定的最大值. 
// 如果 Backoff() 方法没有在 resetDuration 期间调用, backoff 将会被重新设置.
// 当上游不健康时, NewExponentialBackoffManager 可以用来减轻负载
func NewExponentialBackoffManager(initBackoff, maxBackoff, resetDuration time.Duration, backoffFactor, jitter float64, c clock.Clock) BackoffManager {
	return &exponentialBackoffManagerImpl{
		backoff: &Backoff{
			Duration: initBackoff,
			Factor:   backoffFactor,
			Jitter:   jitter,
 
            // 目前的实现, 一旦 steps 被用完, 就会直接返回 Backoff.Duration.
            // 这里根据当前的需求, 不想用完 steps, 随意设置了一个最大的 32 位整数
			Steps: math.MaxInt32,
			Cap:   maxBackoff,
		},
		backoffTimer:         nil,
		initialBackoff:       initBackoff,
		lastBackoffStart:     c.Now(),
		backoffResetDuration: resetDuration,
		clock:                c,
	}
}
```


```go
// 实现了 Backoff() 接口, 调用者可以阻塞的时间可以随指数增长
// 返回的 timer 必须在第二次调用 Backoff() 之前销毁
func (b *exponentialBackoffManagerImpl) Backoff() clock.Timer {
	if b.backoffTimer == nil {
		b.backoffTimer = b.clock.NewTimer(b.getNextBackoff())
	} else {
		b.backoffTimer.Reset(b.getNextBackoff())
	}
	return b.backoffTimer
}
```

```go
func (b *exponentialBackoffManagerImpl) getNextBackoff() time.Duration {
    // b.clock.Now().Sub(b.lastBackoffStart) 计算的是两次 Backoff() 调用时间的差.
    // exponentialBackoffManagerImpl 刚创建时设置 lastBackoffStart 为 time.Now(),
    // 所以 第一次调用 getNextBackoff() 计算的 b.clock.Now().Sub(b.lastBackoffStart) 
	// 是当前时间与 exponentialBackoffManagerImpl 创建时间之差.
 
	// 如果 两次调用 Backoff() 的时间差大于 backoffResetDuration, 重置 backoff.
	if b.clock.Now().Sub(b.lastBackoffStart) > b.backoffResetDuration {
        // 重置 steps = math.MaxInt32, 见 NewExponentialBackoffManager() 分析
		b.backoff.Steps = math.MaxInt32
        // 将 backoff.Duration 设置为初始值
		b.backoff.Duration = b.initialBackoff
	}
	b.lastBackoffStart = b.clock.Now() // 重置 lastBackoffStart
   
    // 通过 backoff.step() 方法返回一个指数增长的 duration.
	return b.backoff.Step()
}
```


```go
// Step (1) returns an amount of time to sleep determined by the
// original Duration and Jitter and (2) mutates the provided Backoff
// to update its Steps and Duration.
func (b *Backoff) Step() time.Duration {
    // 如果 steps 不够用了, 则直接返回, 返回的 duration 根据 jitter 参数设置波动
	if b.Steps < 1 {
		if b.Jitter > 0 {
			return Jitter(b.Duration, b.Jitter)
		}
		return b.Duration
	}
    // steps 参数大于等于1, 即还可以进行一次指数增长
	b.Steps--

	duration := b.Duration

    // 计算指数增长,
    // 如果 factor = 0, 直接使用 duration
	if b.Factor != 0 {
        // 计算 duration = duration * factor
        // 可见 factor = 0 or = 1 效果是一样的
		b.Duration = time.Duration(float64(b.Duration) * b.Factor)
		if b.Cap > 0 && b.Duration > b.Cap {
            // cap 需大于 0
            // 如果 duration 超过了 cap (NewExponentialBackoffManager 设置的最大 maxBackoff),
            // 设置 Duration = Cap 和 step = 0, 即
            // 当下一次执行 backoff.step(), 直接返回 cap.
			b.Duration = b.Cap
			b.Steps = 0
		}
	}

    // 根据 jitter 波动参数设置
	if b.Jitter > 0 {
		duration = Jitter(duration, b.Jitter)
	}
	return duration
}
```
