package egdb

import (
	"context"
	"os"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

// Cache requests that the analytics database of this workload is copied into the cache directory when the
// workload succeeds, making it available to [Latest] in later runs of the workload.
// it is intended to be the last operation of the workload, but only records the request, the copy happens
// once the runner has completed the workload.
func Cache(ctx context.Context, _ eg.Op) error {
	if err := os.WriteFile(egenv.RuntimeDirectory(_eg.AnalyticsCacheRequest), nil, 0600); err != nil {
		return errorsx.Wrap(err, "unable to request caching of the analytics database")
	}

	return nil
}
