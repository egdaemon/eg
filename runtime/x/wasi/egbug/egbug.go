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
)

const nilUUID = "00000000-0000-0000-0000-000000000000"

func Depth() int {
	return env.Int(-1, _eg.EnvComputeModuleNestedLevel)
}

func CachedID() string {
	return env.String(nilUUID, _eg.EnvUnsafeCacheID)
}

// prints debugging information about the current user.
func Users(ctx context.Context, op eg.Op) error {
	privileged := shell.Runtime().Privileged()
	return shell.Run(
		ctx,
		privileged.New("id"),
		privileged.New("id -u egd"),
		privileged.New("users"),
		privileged.New("groups"),
		privileged.New("cat /proc/self/uid_map"),
		privileged.New("cat /proc/self/gid_map"),
		privileged.New("ls -lha /usr/lib/tmpfiles.d"),
		privileged.New("cat /usr/lib/tmpfiles.d/00-eg-daemon.conf"),
		privileged.New("cat /usr/lib/tmpfiles.d/*"),
	)
}

// prints debugging information about the currently executing module.
func Module(ctx context.Context, op eg.Op) error {
	privileged := shell.Runtime().Privileged()
	return shell.Run(
		ctx,
		privileged.Newf("echo run id      : %s", env.String(nilUUID, _eg.EnvComputeRunID)),
		privileged.Newf("echo account id  : %s", env.String(nilUUID, _eg.EnvComputeAccountID)),
		privileged.Newf("echo module depth: %d", Depth()),
		privileged.Newf("echo module log  : %d", env.Int(-1, _eg.EnvComputeLoggingVerbosity)),
	)
}

// prints debugging information about the environment workspaces.
func FileTree(ctx context.Context, op eg.Op) error {
	privileged := shell.Runtime().Privileged()
	return shell.Run(
		ctx,
		privileged.New("ls -lhan /opt"),
		privileged.New("tree -a -L 1 /opt"),
		privileged.Newf("tree -a -L 1 %s", egenv.CacheDirectory()),
		privileged.Newf("tree -a -L 2 %s", egenv.RuntimeDirectory()),
	)
}

// prints current environment variables.
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
		if c := t.Current(); c != v {
			return fmt.Errorf("expected counter to have %d, actual: %d", v, c)
		}

		return nil
	}
}
