// Package egbug is an internal package and not under the compatiability promises of EG.
// its used for debugging and analysis.
package egbug

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

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
		shell.New("id"),
		privileged.New("id"),
		privileged.New("users"),
		privileged.New("groups"),
		privileged.New("cat /proc/self/uid_map"),
		privileged.New("cat /proc/self/gid_map"),
		shell.New("groups"),
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
	privileged := shell.Runtime().Privileged().Lenient(true).Directory("/")
	return shell.Run(
		ctx,
		privileged.Newf("echo 'runtime directory:' && ls -lhan %s", _eg.DefaultMountRoot(_eg.RuntimeDirectory)),
		privileged.Newf("echo 'mount directory:' && ls -lhan %s", _eg.DefaultMountRoot()),
		privileged.Newf("echo 'workload directory:' && ls -lhan %s", _eg.DefaultWorkloadRoot()),
		privileged.Newf("echo 'cache directory:' && ls -lhan %s", egenv.CacheDirectory()),
		privileged.Newf("echo 'ephemeral directory:' && ls -lhan %s", egenv.EphemeralDirectory()),
		privileged.Newf("echo 'working directory:' && ls -lhan %s", egenv.WorkingDirectory()),
		privileged.Newf("tree -a -L 1 %s", egenv.CacheDirectory()),
		privileged.Newf("tree -a -L 1 %s", egenv.EphemeralDirectory()),
		privileged.Newf("tree -a -L 1 %s", egenv.WorkingDirectory()),
		privileged.Newf("tree -a -L 1 %s", _eg.DefaultMountRoot()),
	)
}

// prints current environment variables.
func Env(ctx context.Context, op eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("env | sort"),
		shell.New("env | sort | md5sum"),
	)
}

// debugging information for system initialization
func SystemInit(ctx context.Context, op eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("systemctl list-units --failed").Privileged().Timeout(time.Second),
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

// utility function for prototyping.
func Sleep(d time.Duration) eg.OpFn {
	return func(ctx context.Context, op eg.Op) error {
		time.Sleep(d)
		return nil
	}
}

func Log(m ...any) eg.OpFn {
	return func(ctx context.Context, op eg.Op) error {
		log.Println(m...)
		return nil
	}
}
