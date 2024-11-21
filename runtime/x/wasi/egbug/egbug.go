package egbug

import (
	"context"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/gofrs/uuid"
)

func Depth() int {
	return env.Int(-1, _eg.EnvComputeModuleNestedLevel)
}

func Module(ctx context.Context, op eg.Op) error {
	return shell.Run(
		ctx,
		shell.Newf("echo run id      : %s", env.String(uuid.Nil.String(), _eg.EnvComputeRunID)),
		shell.Newf("echo account id  : %s", env.String(uuid.Nil.String(), _eg.EnvComputeAccountID)),
		shell.Newf("echo module depth: %d", Depth()),
		shell.Newf("echo module log  : %d", env.Int(-1, _eg.EnvComputeLoggingVerbosity)),
	)
}

func FileTree(ctx context.Context, op eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("tree -a -L 1 /cache"),
		shell.New("tree -a -L 1 /opt"),
		shell.New("tree -a -L 2 /opt/egruntime"),
	)
}

func Env(ctx context.Context, op eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("env"),
	)
}

func Images(ctx context.Context, op eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("podman images"),
	)
}
