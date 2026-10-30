package daemon

import (
	"eg/compute/eghoneybadger"
	"eg/compute/faultharness"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egautogen"
)

// Faulttest generates a test case reproducing each of the recent unresolved errors reported to honeybadger,
// as configured by opts, e.g.) the judge and the number of attempts, and pushes them to a branch per fault.
// the project and credentials come from the environment, see eghoneybadger.
func Faulttest(opts egautogen.Options) eg.OpFn {
	return faultharness.Test(1, opts, eghoneybadger.New().Limit(10).Faults())
}
