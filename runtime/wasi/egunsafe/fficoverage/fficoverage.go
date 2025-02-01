package fficoverage

import (
	"context"
	"encoding/json"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/events"
)

func Report(ctx context.Context, batch ...*events.Coverage) (err error) {
	var (
		encoded []byte
	)

	if encoded, err = json.Marshal(batch); err != nil {
		return errorsx.Wrap(err, "unable to marshal payload")
	}
	_ = encoded

	// payloadoffset, payloadlen := ffiguest.Bytes(encoded)

	// return ffiguest.Error(record(ffiguest.ContextDeadline(ctx), payloadoffset, payloadlen), fmt.Errorf("unable to record coverage"))
	return nil
}
