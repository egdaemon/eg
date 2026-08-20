package daemon

import (
	"context"
	"time"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

func shellruntime() shell.Command {
	return shell.Runtime().
		EnvironFrom(eggolang.Env()...).
		Environ(
			"CACHE_DIRECTORY", egenv.CacheDirectory(),
		)
}

func Gogen(ctx context.Context, _ eg.Op) error {
	gruntime := shellruntime()
	return shell.Run(
		ctx,
		gruntime.New("go generate ./... && go fmt ./...").Timeout(40*time.Minute),
	)
}
