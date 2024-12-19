package egbug

import (
	"context"
	"fmt"
	"sync/atomic"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
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
		shell.New("tree -a -L 1 /opt"),
		shell.Newf("tree -a -L 1 %s", egenv.CacheDirectory()),
		shell.Newf("tree -a -L 2 %s", egenv.RuntimeDirectory()),
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

func NewCounter() *counter {
	return langx.Autoptr(counter(0))
}

type counter uint64

func (t *counter) Op(ctx context.Context, op eg.Op) error {
	atomic.AddUint64((*uint64)(t), 1)
	return nil
}

func (t *counter) Current() uint64 {
	return uint64(*t)
}

func (t *counter) Assert(v uint64) eg.OpFn {
	return func(ctx context.Context, op eg.Op) error {
		if c := t.Current(); c != 1 {
			return fmt.Errorf("expected counter to have %d, actual: %d\n", v, c)
		}

		return nil
	}
}
