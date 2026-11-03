package shell

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/interp/execproxy/v2"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/egdaemon/sh/v3/interp"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// serve binds a v2 exec proxy, rooted at work within a new runtime directory, to the runtime directory's control socket.
func serve(t *testing.T) (runtimedir string) {
	var (
		err error
		l   net.Listener
	)

	// unix socket paths are length limited, t.TempDir can exceed it.
	runtimedir, err = os.MkdirTemp("", "shell")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(runtimedir) })
	require.NoError(t, os.Mkdir(filepath.Join(runtimedir, "work"), 0700))

	l, err = net.Listen("unix", filepath.Join(runtimedir, eg.SocketControl))
	require.NoError(t, err)
	srv := grpc.NewServer()
	execproxy.NewProxy(filepath.Join(runtimedir, "work"), runtimedir, os.Environ()).Bind(srv)
	go srv.Serve(l)
	t.Cleanup(srv.Stop)
	return runtimedir
}

func TestRun(t *testing.T) {
	// native points the guest helpers at a served proxy, so Run executes in this process as it would in a guest.
	native := func(t *testing.T) (workdir string) {
		runtimedir := serve(t)
		workdir = filepath.Join(runtimedir, "work")
		t.Setenv(eg.EnvComputeRuntimeDirectory, runtimedir)
		t.Setenv(eg.EnvComputeModuleSocket, filepath.Join(runtimedir, eg.SocketControl))
		t.Setenv(eg.EnvComputeWorkingDirectory, workdir)
		return workdir
	}

	read := func(t *testing.T, path string) string {
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(b)
	}

	t.Run("pipes a builtin into a host command", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, Run(t.Context(), "", "", "echo hello | tr a-z A-Z > out", "", nil))
		require.Equal(t, "HELLO\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("substitutes the output of host commands", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, Run(t.Context(), "", "", `n=$(printf abc | wc -c); echo $((n + 1)) > out`, "", nil))
		require.Equal(t, "4\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("reports a host command's exit code", func(t *testing.T) {
		native(t)
		err := Run(t.Context(), "", "", "sh -c 'exit 3'", "", nil)
		status, ok := errors.AsType[interp.ExitStatus](err)
		require.True(t, ok, err)
		require.Equal(t, interp.ExitStatus(3), status)
	})

	t.Run("a failing host command reaches ||", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, Run(t.Context(), "", "", "ls does-not-exist 2>/dev/null || echo fallback > out", "", nil))
		require.Equal(t, "fallback\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("sends the command's environment and exported variables to the host", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, Run(t.Context(), "", "", `export B=2; sh -c 'echo "$A $B"' > out`, "", []string{"A=1"}))
		require.Equal(t, "1 2\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("runs host commands in the directory relative to the working directory", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, os.Mkdir(filepath.Join(workdir, "sub"), 0700))
		require.NoError(t, Run(t.Context(), "", "", "sh -c 'pwd -P' > ../out", "sub", nil))
		expected, err := filepath.EvalSymlinks(filepath.Join(workdir, "sub"))
		require.NoError(t, err)
		require.Equal(t, expected+"\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("cd moves later host commands", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, os.Mkdir(filepath.Join(workdir, "sub"), 0700))
		require.NoError(t, Run(t.Context(), "", "", "cd sub && touch made", "", nil))
		require.FileExists(t, filepath.Join(workdir, "sub", "made"))
	})

	t.Run("invalid syntax", func(t *testing.T) {
		native(t)
		require.Error(t, Run(t.Context(), "", "", "if then", "", nil))
	})

	t.Run("cancellation stops the host command", func(t *testing.T) {
		native(t)
		ctx, done := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer done()
		ts := time.Now()
		require.Error(t, Run(ctx, "", "", "sleep 30", "", nil))
		require.Less(t, time.Since(ts), 5*time.Second)
	})

	t.Run("wasip1 guest", func(t *testing.T) {
		if testing.Short() {
			t.Skip("builds and runs a wasip1 guest")
		}

		wasm := interptest.Build(t, "./runtime/wasi/shell/v2/.fixtures/guest")
		for _, mode := range []string{"pipeline", "substitution", "devnull", "exit"} {
			t.Run(mode, func(t *testing.T) {
				require.NoError(t, interptest.Run(t, wasm, serve(t), time.Minute, "SHELL_MODE="+mode, eg.EnvComputeWorkingDirectory+"="+eg.DefaultRuntimeDirectory("work")))
			})
		}
	})
}
