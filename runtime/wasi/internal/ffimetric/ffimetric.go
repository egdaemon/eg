package ffimetric

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
)

func Record(ctx context.Context, name string, payload any) (err error) {
	var (
		encoded []byte
	)

	if encoded, err = json.Marshal(payload); err != nil {
		return errorsx.Wrap(err, "unable to marshal payload")
	}

	nameoffset, namelen := ffiguest.String(name)
	payloadoffset, payloadlen := ffiguest.Bytes(encoded)

	return ffiguest.Error(record(ffiguest.ContextDeadline(ctx), nameoffset, namelen, payloadoffset, payloadlen), fmt.Errorf("unable to record metric"))
}
