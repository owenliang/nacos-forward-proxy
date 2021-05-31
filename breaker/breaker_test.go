package breaker

import (
	"fmt"
	"testing"
	"time"
)

func TestDiscovery(t *testing.T) {
	breaker := NewBreaker(&Options{
		DisonnectPeriod:      5 * time.Second,
		RecoverySuccessTimes: 100,
		WindowSize:           10,
		DecideToDisconnect: func(buckets []*Bucket) bool {
			success := 0
			fail := 0
			for _, b := range buckets {
				success += b.Success
				fail += b.Fail
			}
			if fail > success { // 失败比成功还多，那么熔断
				fmt.Println("熔断它！")
				return true
			}
			fmt.Println("不值得熔断~")
			return false
		},
	})

	// 全连接状态
	if breaker.status != BREAKER_STATUS_CONNECT {
		t.Fatal()
	}

	breaker.RecordSuccess()
	breaker.Debug()
	time.Sleep(1 * time.Second)

	breaker.RecordSuccess()
	breaker.Debug()
	time.Sleep(1 * time.Second)

	breaker.RecordFail()
	breaker.RecordFail()
	breaker.RecordFail()
	breaker.Debug()
	// 全断开状态
	if breaker.Status() != BREAKER_STATUS_DISCONNECT {
		t.Fatal()
	}

	// 等5秒，进入半连接状态
	time.Sleep(5 * time.Second)
	if breaker.Status() != BREAKER_STATUS_HALF_CONNECT {
		t.Fatal()
	}

	// 再失败1次，重新回到全断开状态
	breaker.RecordFail()
	if breaker.Status() != BREAKER_STATUS_DISCONNECT {
		t.Fatal()
	}

	// 等5秒，进入半连接状态
	time.Sleep(5 * time.Second)
	if breaker.Status() != BREAKER_STATUS_HALF_CONNECT {
		t.Fatal()
	}

	// 连续成功101次，重回全连接状态
	for i := 0; i < 101; i++ {
		breaker.RecordSuccess()
	}
	if breaker.Status() != BREAKER_STATUS_CONNECT {
		t.Fatal()
	}
}
