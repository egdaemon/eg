// Package egpool runs operations with bounded concurrency. it is a sibling of eg.Sequential and eg.Parallel
// for when the number of operations is large, or unknown until the workload runs, and only some may run at once.
package egpool

import (
	"context"

	"github.com/egdaemon/eg/internal/asynccompute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
)

// Seq is a sequence of items, it is what Pool.Run consumes.
type Seq[T any] = iterx.Seq[T]

// Slice is a Seq of the given items, in order.
func Slice[T any](items ...T) Seq[T] {
	return iterx.Slice(items...)
}

// Func is a Seq of the items fn yields, it is called when the pool runs not when the Seq is created,
// so it can depend on what earlier operations did. fn stops when yield returns false,
// the error it returns is the error of the Seq.
func Func[T any](fn func(ctx context.Context, yield func(T) bool) error) Seq[T] {
	return iterx.New(fn)
}

// Pool runs an operation for each item of a sequence with at most size of them in flight.
type Pool[T any] struct {
	size uint16
	op   func(T) eg.OpFn
}

// New creates a pool that runs at most size operations at once, a size of zero is one.
// op creates the operation for an item, it is called by the worker that runs the operation.
func New[T any](size uint16, op func(T) eg.OpFn) Pool[T] {
	return Pool[T]{size: max(size, 1), op: op}
}

// Run runs an operation for each item of seq, at most the size of the pool at a time.
//
// items are pulled from seq as workers become available, so seq may be lazy.
// like eg.Parallel, a failing operation doesn't stop the others, the first failure is returned,
// along with the error of seq, once everything that was started has finished. later failures are logged.
func (t Pool[T]) Run(seq Seq[T]) eg.OpFn {
	return func(ctx context.Context, op eg.Op) (err error) {
		workers := asynccompute.New(
			func(ctx context.Context, item T) error {
				// sequential for its tracing of the operation.
				return eg.Sequential(t.op(item))(ctx, op)
			},
			asynccompute.Workers[T](t.size),
			asynccompute.Backlog[T](t.size),
		)

		var queued error
		for item := range seq.Each(ctx) {
			if queued = workers.Run(ctx, item); queued != nil {
				break
			}
		}

		// always wait for the started operations, otherwise they outlive the pool.
		return errorsx.Compact(queued, workers.Close(), seq.Err())
	}
}
