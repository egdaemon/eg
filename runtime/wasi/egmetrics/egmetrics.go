package egmetrics

import (
	"context"

	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffimetric"
)

// Records a metric payload for the given name. the name
// field is an opaque string identifying the metric.
// the payload must be consistent with field name -> types.
// the prefix 'eg.' is reserved for system use.
// e.g.) ✓ Record(ctx, "example.metric.1", m)
// e.g.) x Record(ctx, "eg.example.metric.1", m)
// metrics will be encoded as follows:
//   - every event will be given a uuid v7 id.
//   - every event will be given a timestamp.
//   - name will be recorded as its literal value and as a hashed uuid.
//   - timestamps must be encoded as ISO8601.
func Record(ctx context.Context, name string, m any) error {
	return ffimetric.Record(ctx, name, m)
}
