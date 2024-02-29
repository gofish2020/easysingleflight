package easysingleflight

import (
	"bytes"
	"fmt"
	"runtime/debug"
	"sync"
)

type (
	call struct {
		done chan struct{} // 处理完成的通知通道

		val interface{} // 结果
		err error       // 是否出错
	}

	Group struct {
		calls map[string]*call // 懒初始化
		mu    sync.Mutex
	}
)

func NewSingleFlight() *Group {
	return &Group{}
}

// bool值表示：本次返回值是缓存值，还是实际走的fn函数
func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error, bool) {

	// 1.加锁
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[string]*call)
	}

	// 2.key存在，说明有重复的调用，只能等待...
	if c, ok := g.calls[key]; ok {
		g.mu.Unlock()
		<-c.done // 阻塞等待结果中..
		return c.val, c.err, true
	}

	// 3.key不存在，说明这是第一个调用
	cl := new(call)
	cl.done = make(chan struct{})
	g.calls[key] = cl
	// 4. 在执行fn前解锁
	g.mu.Unlock()

	func() {
		//defer 目的避免fn出现panic
		defer func() {

			if p := recover(); p != nil {
				cl.err = newPanicError(p)
				cl.val = nil
			}

			// 6. 删除key
			g.mu.Lock()
			delete(g.calls, key)
			g.mu.Unlock()
			// 7.通知阻塞在该call的协程,结束阻塞（呼应上面的阻塞）
			close(cl.done)
		}()
		// 5.执行fn
		cl.val, cl.err = fn()
	}()

	return cl.val, cl.err, false
}

type panicError struct {
	value interface{}
	stack []byte
}

func (p *panicError) Error() string {
	return fmt.Sprintf("%v\n\n%s", p.value, p.stack)
}

func (p *panicError) Unwrap() error {
	err, ok := p.value.(error)
	if !ok {
		return nil
	}
	return err
}

func newPanicError(v interface{}) error {
	stack := debug.Stack()

	if line := bytes.IndexByte(stack[:], '\n'); line >= 0 {
		stack = stack[line+1:]
	}
	return &panicError{value: v, stack: stack}
}
