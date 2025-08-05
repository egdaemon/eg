package ffigraph

import (
	"context"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

type node interface {
	ID() string
}

type Traceable interface {
	Tracer() Eventer
}

func tracer(n node) Eventer {
	if t, ok := n.(Traceable); ok {
		return t.Tracer()
	}

	if t, ok := n.(Eventer); ok {
		return t
	}

	return nil
}

type Eventer interface {
	OpInfo(ts time.Time, cause error, path []string) *events.Op
}

type path []string

type keys int

const (
	contextkey keys = iota
)

func pushv0(ctx context.Context, n node, fn func(ctx context.Context) error) (err error) {
	np := tracer(n)
	if np == nil {
		// nothing to trace
		return fn(ctx)
	}

	current, _ := ctx.Value(contextkey).(path)
	latest := append(current, n.ID())
	dctx := context.WithValue(ctx, contextkey, latest)
	ts := time.Now()
	defer func() {
		errorsx.Log(recordevt(ctx, np.OpInfo(ts, err, current)))
	}()
	return fn(dctx)
}

func TraceErr(ctx context.Context, op node, fn func(ctx context.Context) error) error {
	return pushv0(ctx, op, fn)
}

func Wrap(ctx context.Context, op node, fn func(ctx context.Context)) {
	_ = pushv0(ctx, op, func(ctx context.Context) error {
		fn(ctx)
		return nil
	})
}

func recordevt(ctx context.Context, op *events.Op) (err error) {
	if op == nil {
		return nil
	}

	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	d := events.NewEventsClient(cc)

	if _, err = d.Dispatch(ctx, events.NewDispatch(events.NewOp(op))); err != nil {
		return errorsx.Wrap(err, "unable to record graph metric")
	}

	return nil
}
