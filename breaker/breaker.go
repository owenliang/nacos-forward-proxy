package breaker

import (
	"fmt"
	"sync"
	"time"
)

const (
	BREAKER_STATUS_CONNECT      int = iota // 全连接
	BREAKER_STATUS_DISCONNECT              // 全断开
	BREAKER_STATUS_HALF_CONNECT            // 半连接
)

type Bucket struct {
	Success int
	Fail    int
}

// 熔断器
type Breaker struct {
	mu                     sync.Mutex
	status                 int                  // 熔断状态
	disonnectPeriod        time.Duration        // 全断开的持续时间
	recoverySuccessTimes   int                  // "半连接"需要连续成功recoverySuccessTimes次才能恢复到"全连接"状态
	buckets                []*Bucket            // 秒级统计（1秒1个桶），维护最近N秒的数据
	lastUpdateTime         time.Time            // 最近1次更新时间
	disconnectTime         time.Time            // 发生"全断开"的时间点
	recoverySuccessCounter int                  // 统计半连接期间的连续成功次数
	decideToDisconnect     func([]*Bucket) bool // 用户传入，决定是否进入"全断开"状态
}

type Options struct {
	DisonnectPeriod      time.Duration        // 全断开的持续时间
	RecoverySuccessTimes int                  // "半连接"需要连续成功recoverySuccessTimes次才能恢复到"全连接"状态
	WindowSize           int                  // 窗口大小（多少秒）
	DecideToDisconnect   func([]*Bucket) bool // 用户传入，决定是否进入"全断开"状态
}

func NewBreaker(options *Options) (breaker *Breaker) {
	breaker = &Breaker{
		status:               BREAKER_STATUS_CONNECT,
		disonnectPeriod:      options.DisonnectPeriod,
		recoverySuccessTimes: options.RecoverySuccessTimes,
		buckets:              make([]*Bucket, 0),
		lastUpdateTime:       time.Now(),
		decideToDisconnect:   options.DecideToDisconnect,
	}
	for i := 0; i < options.WindowSize; i++ {
		breaker.buckets = append(breaker.buckets, &Bucket{})
	}
	return
}

func (breaker *Breaker) Status() int {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()
	breaker.update()
	return breaker.status
}

func (breaker *Breaker) Debug() {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	breaker.update()
	fmt.Println("-----status:", breaker.status, "-------")
	for i := 0; i < len(breaker.buckets); i++ {
		fmt.Printf("bucket[%d] success=%d fail=%d\n", i, breaker.buckets[i].Success, breaker.buckets[i].Fail)
	}
	fmt.Println("------")
}

func (breaker *Breaker) update() {
	now := time.Now()

	// 桶偏移
	secs := int(now.Sub(breaker.lastUpdateTime).Seconds())
	if secs >= len(breaker.buckets) { // 全部过去了，清空就行，免得动内存
		for _, bucket := range breaker.buckets {
			bucket.Success = 0
			bucket.Fail = 0
		}
	} else if secs > 0 {
		// 淘汰过期时间的桶
		breaker.buckets = append(breaker.buckets[:0], breaker.buckets[secs:]...)
		// 向后追加一些新桶
		for i := 0; i < secs; i++ {
			breaker.buckets = append(breaker.buckets, &Bucket{})
		}
	}

	// "完全断开"状态下，度过一定的时间后应该进入"半连接"状态
	if breaker.status == BREAKER_STATUS_DISCONNECT {
		if now.Sub(breaker.disconnectTime) > breaker.disonnectPeriod {
			breaker.status = BREAKER_STATUS_HALF_CONNECT
			breaker.recoverySuccessCounter = 0
		}
	}
	breaker.lastUpdateTime = now
}

// 记录成功
func (breaker *Breaker) recordSuccess() {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	breaker.update()
	breaker.buckets[len(breaker.buckets)-1].Success++
	if breaker.status == BREAKER_STATUS_HALF_CONNECT { // "半连接"，需要突破连续成功次数，才能恢复至"全连接"
		breaker.recoverySuccessCounter++
		if breaker.recoverySuccessCounter > breaker.recoverySuccessTimes { // 突破成功，状态转换“全连接”
			breaker.status = BREAKER_STATUS_CONNECT
			// 清空计数器，重新做人
			for _, b := range breaker.buckets {
				b.Success = 0
				b.Fail = 0
			}
		}
	}
}

// 记录失败
func (breaker *Breaker) recordFail() {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	breaker.update()
	breaker.buckets[len(breaker.buckets)-1].Fail++

	needDisconnect := false
	if breaker.status == BREAKER_STATUS_HALF_CONNECT { // "半连接", 遇到错误直接转回至 "全断开"
		needDisconnect = true
	} else if breaker.status == BREAKER_STATUS_CONNECT { // "全连接"，那么让用户判断是否要"全断开"
		if cut := breaker.decideToDisconnect(breaker.buckets); cut { // 用户决定"全断开"
			needDisconnect = true
		}
	}

	// 熔断
	if needDisconnect {
		breaker.status = BREAKER_STATUS_DISCONNECT
		breaker.disconnectTime = time.Now()
	}
}

// 是否允许操作？
func (breaker *Breaker) ok() bool {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	breaker.update()
	if breaker.status == BREAKER_STATUS_DISCONNECT {
		return false
	}
	return true
}
