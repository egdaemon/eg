package fficoverage

import (
	"context"
	"encoding/json"
	"log"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/tetratelabs/wazero/api"
)

type tracefnv0 func(
	ctx context.Context,
	m api.Module,
	deadline int64, // context.Context
	jsonoffset uint32, jsonlen uint32, // []byte
) uint32

func Report(d events.EventsClient) tracefnv0 {
	return func(
		ctx context.Context,
		m api.Module,
		deadline int64, // context.Context
		jsonoffset uint32, jsonlen uint32, // []byte
	) uint32 {
		var (
			err     error
			encoded []byte
			op      []*events.Coverage
		)

		ctx, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		if encoded, err = ffi.ReadBytes(m.Memory(), jsonoffset, jsonlen); err != nil {
			log.Println(errorsx.Wrap(err, "unable to read fields argument"))
			return 1
		}

		if err = json.Unmarshal(encoded, &op); err != nil {
			return 1
		}

		if _, err = d.Dispatch(ctx, events.NewDispatch(slicesx.MapTransform(func(rep *events.Coverage) *events.Message { return events.NewCoverage(rep) }, op...)...)); err != nil {
			log.Println(errorsx.Wrap(err, "unable to write coverage"))
			return 1
		}

		return 0
	}
}
