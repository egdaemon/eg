package daemon

import (
	"eg/compute/eghoneybadger"
	"eg/compute/faultharness"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egautogentest"
)

// Faulttest generates a test case reproducing each of the recent unresolved errors reported to honeybadger,
// as configured by opts, e.g.) the judge and the number of attempts, and pushes them to a branch per fault.
// the project and credentials come from the environment, see eghoneybadger.
func Faulttest(opts egautogentest.Options) eg.OpFn {
	return faultharness.Test(workers, opts, eghoneybadger.New().Limit(10).Faults())
}
