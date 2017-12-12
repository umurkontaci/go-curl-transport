package pool

import (
	"github.com/umurkontaci/go-curl"
	"runtime"
	"sync"
)

// Finalizer is an interface for finalizing curl instances
type Finalizer interface {
	Finalize(c *curlBox)
}

// FinalizingPool is wraps sync.Pool with a mutex for thread safety and encapsulates curl instances in a boxing struct.
// Anything put into sync.Pool can be garbage collected any time, and we don't want curl instances to be garbage
// collected without running curl.Cleanup() on them. The boxing method allows us to wrap the curl instances are set
// finalizers on the box that will clean curl instance.
type FinalizingPool struct {
	pool sync.Pool
	Finalizer
	mutex sync.Mutex
}

type curlBox struct {
	curl *curl.CURL
}

// Get will return a curl instance from the pool or create a new one if it doesn't exist.
func (p *FinalizingPool) Get() *curl.CURL {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	c := p.pool.Get()
	if c != nil {
		container := c.(*curlBox)
		runtime.SetFinalizer(container, nil)
		return container.curl
	}
	newCurl := curl.EasyInit()
	return newCurl
}

// Put will put the curl instance back into the pool so it can be reused or garbage collected as necessary.
func (p *FinalizingPool) Put(c *curl.CURL) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	container := &curlBox{c}
	runtime.SetFinalizer(container, p.Finalize)
	p.pool.Put(container)
}

// Finalize is the finalizer for the curlBox that will cleanup the curl instance.
func (p *FinalizingPool) Finalize(c *curlBox) {
	if p.Finalizer != nil {
		p.Finalizer.Finalize(c)
	} else {
		c.curl.Cleanup()
	}
}
