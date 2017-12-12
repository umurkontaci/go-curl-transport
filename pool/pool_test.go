package pool

import (
	"github.com/umurkontaci/go-curl"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPool_Get(t *testing.T) {
	pool := FinalizingPool{}
	c := pool.Get()
	if c == nil {
		t.Errorf("Get returned nil")
	}
}

type MockFinalizer struct {
	FinalizeHandler func(container *curlBox)
}

func (m *MockFinalizer) Finalize(container *curlBox) {
	m.FinalizeHandler(container)
}

func TestPool_GC_ShouldCallFinalizer(t *testing.T) {
	var count int32
	ws := sync.WaitGroup{}

	mf := MockFinalizer{
		FinalizeHandler: func(container *curlBox) {
			atomic.AddInt32(&count, 1)
			ws.Done()
		},
	}

	func() {
		pool := FinalizingPool{Finalizer: &mf}
		c := make([]*curl.CURL, 100)
		for i := 0; i < 100; i++ {
			c = append(c, pool.Get())
		}
		for i := 0; i < 100; i++ {
			ws.Add(1)
			pool.Put(c[i])

		}
	}()
	runtime.GC()
	c := make(chan bool, 1)
	go func() {
		ws.Wait()
		c <- true
	}()
	select {
	case <-c:
		// noop
	case <-time.After(time.Second * 2):
		log.Fatalf("Timeout while waiting for GC")
		t.Fail()
	}
	if atomic.LoadInt32(&count) < 99 {
		t.Fail()
	}
}
