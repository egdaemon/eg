package ffimetric

import (
	"context"
	"encoding/json"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
)

func Record(ctx context.Context, name string, payload any) (err error) {
	var (
		encoded []byte
	)

	if encoded, err = json.Marshal(payload); err != nil {
		return errorsx.Wrap(err, "unable to marshal metric data")
	}

	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return err
	}
	d := events.NewEventsClient(cc)

	if _, err = d.Dispatch(ctx, events.NewDispatch(events.NewMetric(name, encoded))); err != nil {
		return errorsx.Wrap(err, "unable to record graph metric")
	}

	return nil
}
