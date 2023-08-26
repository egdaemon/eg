package httpx

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"sync"
)

type contextKey int

const (
	contextKeyBufferPool contextKey = iota
)

type bufferTracking struct {
	httputil.BufferPool
	m       sync.Mutex
	buffers [][]byte
}

// ContextBufferPool1024 - convience pool that uses a 1024 byte allocation buffer pool.
func ContextBufferPool1024() func(http.Handler) http.Handler {
	return ContextBufferPool(NewBufferPool(1024))
}

// ContextBufferPool512 - convience pool that uses a 512 byte allocation buffer pool.
func ContextBufferPool512() func(http.Handler) http.Handler {
	return ContextBufferPool(NewBufferPool(512))
}

// ContextBufferPool inserts a buffer into the http.Request context.
func ContextBufferPool(pool httputil.BufferPool) func(http.Handler) http.Handler {
	return func(original http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			tpool := &bufferTracking{
				BufferPool: pool,
				buffers:    make([][]byte, 0, 10),
			}

			original.ServeHTTP(resp, req.WithContext(context.WithValue(req.Context(), contextKeyBufferPool, tpool)))

			for _, buf := range tpool.buffers {
				pool.Put(buf)
			}
		})
	}
}

// GetBuffer generates a resp buffer from an underlying pool if any.
func GetBuffer(req *http.Request) *bytes.Buffer {
	pool, ok := req.Context().Value(contextKeyBufferPool).(*bufferTracking)
	if !ok {
		log.Println(errors.New("request does not contain a pool, generating a new buffer object"))
		return bytes.NewBuffer([]byte{})
	}

	buf := pool.BufferPool.Get()
	pool.m.Lock()
	pool.buffers = append(pool.buffers, buf)
	pool.m.Unlock()

	return bytes.NewBuffer(buf)
}

// ReleaseBuffer releases the buffer back into the underlying pool if any.
func ReleaseBuffer(req *http.Request, buffer *bytes.Buffer) {
	pool, ok := req.Context().Value(contextKeyBufferPool).(httputil.BufferPool)
	if !ok {
		log.Println(errors.New("request does not contain a pool, dropping buffer onto the floor"))
	}
	pool.Put(buffer.Bytes())
}
