// Package asyncx provides primitives for waking background workers.
package asyncx

import (
	"context"
	"log"
	"sync"

	"github.com/egdaemon/eg/internal/errorsx"
)

const (
	ErrWakeupClosed = errorsx.String("wakeup closed")
)

// Wakeup lets producers nudge a consumer. Signal wakes a single waiting
// consumer, Broadcast wakes all of them. Consumers select on C, a nil error
// means woken, ErrWakeupClosed means the wakeup was closed.
type Wakeup struct {
	*sync.Cond
	done    chan struct{}
	C       <-chan error
	cleanup func()
}

func (t *Wakeup) Close() error {
	t.cleanup()
	return nil
}

func NewWakeup(ctx context.Context) *Wakeup {
	ictx, done := context.WithCancel(ctx)
	q := make(chan error, 1)
	wakeup := sync.NewCond(&sync.Mutex{})
	donesig := make(chan struct{})
	a := &Wakeup{
		Cond: wakeup,
		done: donesig,
		C:    q,
		cleanup: sync.OnceFunc(func() {
			done()
			select {
			case q <- ErrWakeupClosed:
				wakeup.Broadcast()
			case <-ctx.Done():
				log.Println("async wakeup parent context was cancelled")
			}

			close(donesig)
		}),
	}

	go func() {
		for {
			a.L.Lock()
			a.Wait()
			a.L.Unlock()

			select {
			case <-ictx.Done():
				return
			case <-donesig:
				return
			case q <- nil:
			}
		}
	}()

	return a
}
