package daemon

import (
	"context"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func Agent(ctx context.Context, _ eg.Op) error {
	return shell.Run(
		ctx,
		shell.Env().New("tail -f /dev/null").Timeout(egenv.TTL()),
	)
}
