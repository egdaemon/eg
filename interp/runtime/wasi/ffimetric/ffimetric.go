package ffimetric

import (
	"context"
	"log"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/tetratelabs/wazero/api"
)

func Metric(
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

	if name, err = ffi.ReadString(m.Memory(), nameoffset, namelen); err != nil {
		log.Println(errorsx.Wrap(err, "unable to read name argument"))
		return 1
	}

	if fields, err = ffi.ReadBytes(m.Memory(), jsonoffset, jsonlen); err != nil {
		log.Println(errorsx.Wrap(err, "unable to read fields argument"))
		return 1
	}

	log.Printf("metric event received '%s' '%s'\n", name, string(fields))

	return 0
}
