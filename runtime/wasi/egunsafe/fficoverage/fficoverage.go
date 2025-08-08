package fficoverage

import (
	"context"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

func Report(ctx context.Context, batch ...*events.Coverage) (err error) {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	d := events.NewEventsClient(cc)

	if _, err = d.Dispatch(ctx, events.NewDispatch(slicesx.MapTransform(func(rep *events.Coverage) *events.Message { return events.NewCoverage(rep) }, batch...)...)); err != nil {
		return errorsx.Wrap(err, "unable to report coverage")
	}
	return nil
}
