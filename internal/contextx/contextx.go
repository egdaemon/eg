package contextx

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

type keys int

const (
	contextKeyWaitgroup keys = iota
)

func Until(ctx context.Context) time.Duration {
	if ts, ok := ctx.Deadline(); ok {
		return time.Until(ts)
	}

	return math.MaxInt64
}

func WithWaitGroup(ctx context.Context, wg *sync.WaitGroup) context.Context {
	return context.WithValue(ctx, contextKeyWaitgroup, wg)
}

// NewWaitGroup - adds a waitgroup to the context.
func NewWaitGroup(ctx context.Context) context.Context {
	return WithWaitGroup(ctx, &sync.WaitGroup{})
}

// WaitGroup - retrieve the waitgroup from the context.
func WaitGroup(ctx context.Context) (*sync.WaitGroup, bool) {
	wg, ok := ctx.Value(contextKeyWaitgroup).(*sync.WaitGroup)
	return wg, ok
}

// WaitGroupAdd - increment the waitgroup by delta.
func WaitGroupAdd(ctx context.Context, delta int) {
	if wg, ok := WaitGroup(ctx); ok {
		wg.Add(delta)
	}
}

// WaitGroupDone - decrement the waitgroup
func WaitGroupDone(ctx context.Context) {
	if wg, ok := WaitGroup(ctx); ok {
		wg.Done()
	}
}

func IgnoreDeadlineExceeded(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return err
}

func IgnoreCancelled(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
