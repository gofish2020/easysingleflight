package easysingleflight

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestExclusiveCallDoDupSuppress(t *testing.T) {
	g := NewSingleFlight()
	c := make(chan string)
	var calls int32

	fn := func() (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return <-c, nil
	}

	const n = 10
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)

		// 1个协程执行fn，9个协程阻塞等待结果
		go func() {
			v, err, _ := g.Do("key", fn)
			if err != nil {
				t.Errorf("Do error: %v", err)
			}
			if v.(string) != "bar" {
				t.Errorf("got %q; want %q", v, "bar")
			}
			wg.Done()

		}()
	}
	time.Sleep(100 * time.Millisecond) // let goroutines above block
	c <- "bar"                         // 让执行fn的协程返回结果，那么阻塞的9个协程才会返回
	wg.Wait()

	// 最后从calls可知，fn真正只执行了1次
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("number of calls = %d; want 1", got)
	}
}
