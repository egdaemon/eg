// Package asynccompute provides a pool of workers for running work in the background.
package asynccompute

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
)

// Pool of workers.
type Pool[T any] struct {
	workers  int
	shutdown sync.WaitGroup // track active compute routines
	async    func(ctx context.Context, w T) error
	queued   chan pending
	failed   atomic.Pointer[error]
}

// Run queues the workload, it blocks while the backlog is full. the context given is the one
// the workload is run with.
func (t *Pool[T]) Run(ctx context.Context, w T) error {
	select {
	case t.queued <- pending{workload: func() error { return t.async(ctx, w) }}:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// Close stops accepting workloads and waits for the queued ones to complete, returning the first failure.
func (t *Pool[T]) Close() error {
	close(t.queued)
	t.shutdown.Wait()
	return langx.Autoderef(t.failed.Load())
}

func (t *Pool[T]) init() *Pool[T] {
	t.shutdown.Add(int(t.workers))
	for range t.workers {
		go func() {
			defer t.shutdown.Done()
			for pending := range t.queued {
				if cause := pending.workload(); cause != nil {
					errorsx.Log(cause)
					t.failed.CompareAndSwap(nil, &cause)
				}
			}
		}()
	}

	return t
}

type pending struct {
	workload func() error
}

type Option[T any] func(*Pool[T])

// Backlog is how many workloads can be queued before Run blocks.
func Backlog[T any](n uint16) Option[T] {
	return func(p *Pool[T]) {
		p.queued = make(chan pending, n)
	}
}

// Workers is how many workloads run concurrently.
func Workers[T any](n uint16) Option[T] {
	return func(p *Pool[T]) {
		p.workers = int(n)
	}
}

func Compose[T any](options ...Option[T]) Option[T] {
	return func(p *Pool[T]) {
		for _, opt := range options {
			opt(p)
		}
	}
}

func New[T any](async func(ctx context.Context, w T) error, options ...Option[T]) *Pool[T] {
	return new(langx.Clone(Pool[T]{
		workers: runtime.NumCPU(),
		queued:  make(chan pending, runtime.NumCPU()),
		async:   async,
	}, options...)).
		init()
}

// gracefully shutdown by invoking close and waiting until all workers
// complete or the context times out.
func Shutdown[T any](ctx context.Context, p *Pool[T]) error {
	dctx, cancelled := context.WithCancelCause(ctx)
	go func() {
		cancelled(p.Close())
	}()

	<-dctx.Done()
	return errorsx.Ignore(context.Cause(dctx), context.Canceled)
}
