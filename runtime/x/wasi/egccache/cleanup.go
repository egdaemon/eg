package egccache

import (
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

type cleanupoption = func(*cleanup)
type cleanupoptions []cleanupoption

func CleanupOption() cleanupoptions {
	return cleanupoptions(nil)
}

// maximum disk usage allowed in cache
func (t cleanupoptions) DiskLimit(max bytesx.Unit) cleanupoptions {
	return append(t, func(b *cleanup) {
		b.Usage = max
	})
}

// maximum disk usage allowed in cache
func (t cleanupoptions) UnsafeRuntime(runtime shell.Command) cleanupoptions {
	return append(t, func(b *cleanup) {
		b.Runtime = runtime
	})
}

type cleanup struct {
	Runtime shell.Command
	Usage   bytesx.Unit
}

func Cleanup(options ...cleanupoption) eg.OpFn {
	cfg := langx.Clone(cleanup{
		Runtime: Runtime(),
		Usage:   bytesx.EiB, // effectively unlimited, dont make users think about this unless they want to.
	}, options...)

	return shell.Op(
		cfg.Runtime.Newf("echo ccache -M %v", cfg.Usage),
	)
}
