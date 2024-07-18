package ffimetric

import (
	"context"
	"log"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/tetratelabs/wazero/api"
)

type metricfnv0 func(
	ctx context.Context,
	m api.Module,
	deadline int64, // context.Context
	nameoffset uint32, namelen uint32, // string
	jsonoffset uint32, jsonlen uint32, // []byte
) uint32

func Metric(d events.EventsClient) metricfnv0 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64, // context.Context
		nameoffset uint32, namelen uint32, // string
		jsonoffset uint32, jsonlen uint32, // []byte
	) uint32 {
		var (
			err    error
			name   string
			fields []byte
		)

		ctx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
			log.Println(errorsx.Wrap(err, "unable to read name argument"))
			return 1
		}

		if fields, err = ffi.ReadBytes(m.Memory(), jsonoffset, jsonlen); err != nil {
			log.Println(errorsx.Wrap(err, "unable to read fields argument"))
			return 1
		}

		if _, err = d.Dispatch(ctx, events.NewDispatch(events.NewMetric(name, fields))); err != nil {
			log.Println(errorsx.Wrap(err, "unable to write metric"))
			return 1
		}

		return 0
	}
}
