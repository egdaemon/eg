// Package egbug is an internal package and not under the compatiability promises of EG.
// its used for debugging and analysis.
package egbug

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/gofrs/uuid"
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

// prints the directory tree of the provided path.
func DirectoryTree(dir string) eg.OpFn {
	return func(ctx context.Context, op eg.Op) error {
		privileged := shell.Runtime().Privileged().Lenient(true).Directory("/")
		return shell.Run(
			ctx,
			privileged.Newf("tree -a %s", dir),
		)
	}
}

func Fail(ctx context.Context, op eg.Op) error {
	return fmt.Errorf("explicitly failing due to egbug.Fail being invoked")
}

// Utility operation for debugging failures
func DebugFailure(op, debug eg.OpFn) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		if err := op(ctx, o); err != nil {
			errorsx.Log(errorsx.Wrap(debug(ctx, o), "debug operation failed"))
			return err
		}

		return nil
	}
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

// Ensures the environment is stable between releases.
func EnsureEnv(ctx context.Context, op eg.Op) error {
	// expected hash with normalized values.
	// if this needs to change it means we might be breaking
	// existing builds.
	const expected = "1e10f3e57339d5194391a968066609d0"
	// zero out some dynamic environment variables for consistent results
	os.Setenv(_eg.EnvComputeAccountID, uuid.Nil.String())
	os.Setenv(_eg.EnvComputeRunID, uuid.Nil.String())
	os.Setenv(_eg.EnvGitHeadCommitTimestamp, "0000-00-00T00:00:00-00:00")
	os.Setenv(_eg.EnvGitHeadCommit, "0000000000000000000000000000000000000000")
	os.Setenv(_eg.EnvGitBaseCommit, "0000000000000000000000000000000000000000")
	os.Setenv(_eg.EnvUnsafeCacheID, uuid.Nil.String())

	environ := os.Environ()
	sort.Strings(environ)

	digest := md5.New()
	for _, v := range environ {
		if _, err := digest.Write([]byte(v)); err != nil {
			return err
		}
	}

	if d := hex.EncodeToString(digest.Sum(nil)); d != expected {
		return fmt.Errorf("unexpected environment digest: %s != %s:\n%s", d, expected, strings.Join(environ, "\n"))
	}

	return nil
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
