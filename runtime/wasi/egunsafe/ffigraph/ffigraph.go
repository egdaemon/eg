package ffigraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
)

type node interface {
	ID() string
}

type root struct{}

func (root) ID() string {
	return ""
}

type path []string

type keys int

const (
	contextkey keys = iota
)

func pushv0(ctx context.Context, n node, fn func(ctx context.Context) error) (err error) {
	current, _ := ctx.Value(contextkey).(path)
	latest := append(current, n.ID())
	dctx := context.WithValue(ctx, contextkey, latest)
	ts := time.Now()
	defer func() {
		recordevt(ctx, &events.Op{
			State:        events.OpState(err),
			Milliseconds: int64(time.Since(ts) / time.Millisecond),
			Name:         n.ID(),
			Path:         current,
		})
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
	var (
		encoded []byte
	)

	if encoded, err = json.Marshal(op); err != nil {
		return err
	}

	deadline := ffiguest.ContextDeadline(ctx)
	opptr, oplen := ffiguest.Bytes(encoded)
	return ffiguest.Error(_recordevt(deadline, opptr, oplen), fmt.Errorf("unable to record op event"))
}
