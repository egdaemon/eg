package httpx

import "sync"

// NewBufferPool pools of buffers that can be reused.
func NewBufferPool(newAllocationSize int) BufferPool {
	return BufferPool{
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, newAllocationSize)
			},
		},
	}
}

// BufferPool implementation.
type BufferPool struct {
	pool *sync.Pool
}

// Get - get a buffer from the pool.
func (t BufferPool) Get() []byte {
	return t.pool.Get().([]byte)
}

// Put - put a buffer into the pool.
func (t BufferPool) Put(b []byte) {
	t.pool.Put(b) // nolint: staticcheck
}
