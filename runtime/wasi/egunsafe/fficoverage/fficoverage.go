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

// Worst returns the n functions with the lowest hit counts.
func Worst(ctx context.Context, n int32) ([]*events.Coverage, error) {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return nil, err
	}
	d := events.NewEventsClient(cc)

	resp, err := d.WorstCoverageFunctions(ctx, &events.WorstCoverageFunctionsRequest{N: n})
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to query worst coverage")
	}

	return resp.Functions, nil
}

// Sample returns a random sample of n functions.
func Sample(ctx context.Context, n int32) ([]*events.Coverage, error) {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return nil, err
	}
	d := events.NewEventsClient(cc)

	resp, err := d.SampleCoverageFunctions(ctx, &events.SampleCoverageFunctionsRequest{N: n})
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to query sample coverage")
	}

	return resp.Functions, nil
}
