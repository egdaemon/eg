package eghoneybadger

import (
	"context"
	"slices"
	"strings"

	"eg/compute/faultharness"

	"github.com/egdaemon/eg/runtime/x/wasi/egpool"
)

// projectroot is how honeybadger prefixes the paths of frames inside the project.
const projectroot = "[PROJECT_ROOT]/"

// Faults is the faultharness source of the recent faults, fetched when the harness runs.
// a fault without a frame in the project, or without a notice, has nothing to reproduce and is skipped.
func (t client) Faults() faultharness.Seq {
	return egpool.Func(func(ctx context.Context, yield func(faultharness.T) bool) error {
		faults, err := t.Recent(ctx)
		if err != nil {
			return err
		}

		for _, hf := range faults {
			n, err := t.Latest(ctx, hf)
			if err != nil {
				return err
			}

			if n == nil {
				continue
			}

			i := slices.IndexFunc(n.Backtrace, func(f Frame) bool {
				return strings.HasPrefix(f.File, projectroot)
			})
			if i < 0 {
				continue
			}

			top := n.Backtrace[i]
			f := faultharness.T{
				ID:         hf.ID.String(),
				Stacktrace: hf.Klass + ": " + hf.Message + "\n\n" + n.String(),
				Path:       strings.TrimPrefix(top.File, projectroot),
				Func:       top.Method,
				Line:       int(top.Number),
			}

			if !yield(f) {
				return nil
			}
		}

		return nil
	})
}
