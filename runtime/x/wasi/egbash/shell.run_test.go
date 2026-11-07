package egbash

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

	// guest points the guest helpers at a module that does not exist, so a command only succeeds if it never
	// leaves the guest.
	guest := func(t *testing.T) (workdir string) {
		workdir = t.TempDir()
		t.Setenv(eg.EnvComputeRuntimeDirectory, workdir)
		t.Setenv(eg.EnvComputeModuleSocket, filepath.Join(workdir, "missing.socket"))
		t.Setenv(eg.EnvComputeWorkingDirectory, workdir)
		return workdir
	}

	read := func(t *testing.T, path string) string {
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(b)
	}

	t.Run("coreutils run in the guest", func(t *testing.T) {
		workdir := guest(t)
		require.NoError(t, Run(t.Context(), New("echo hello world | cat > out")))
		require.Equal(t, "hello world\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("coreutils manage files in the guest", func(t *testing.T) {
		workdir := guest(t)
		require.NoError(t, Run(t.Context(), New("mkdir -p a/b && echo hi > a/b/f && cp a/b/f c && mv c out && rm -r a")))
		require.Equal(t, "hi\n", read(t, filepath.Join(workdir, "out")))
		require.NoDirExists(t, filepath.Join(workdir, "a"))
	})

	// the flags below are ones eg's packages and workloads pass. u-root does not support them yet, so these are
	// skipped until it does, or until the commands stop going to x/coreutils.
	t.Run("tar archives a directory with -C", func(t *testing.T) {
		t.Skip("u-root's tar has no -C: egtarball and release/eg/targz pass it")
		workdir := guest(t)
		require.NoError(t, Run(t.Context(), New("mkdir -p src out && echo hi > src/f && tar -C src -czf a.tgz . && tar -C out -xzf a.tgz")))
		require.Equal(t, "hi\n", read(t, filepath.Join(workdir, "out", "f")))
	})

	t.Run("cp -a copies a directory with its modes", func(t *testing.T) {
		t.Skip("u-root's cp has no -a")
		workdir := guest(t)
		require.NoError(t, os.MkdirAll(filepath.Join(workdir, "src"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(workdir, "src", "f"), []byte("hi"), 0750))
		require.NoError(t, Run(t.Context(), New("cp -a src dst")))
		info, err := os.Stat(filepath.Join(workdir, "dst", "f"))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0750), info.Mode().Perm())
	})

	t.Run("chmod -R changes a directory's tree", func(t *testing.T) {
		t.Skip("u-root's chmod has no -R")
		workdir := guest(t)
		require.NoError(t, os.MkdirAll(filepath.Join(workdir, "src", "sub"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(workdir, "src", "sub", "f"), []byte("hi"), 0600))
		require.NoError(t, Run(t.Context(), New("chmod -R 755 src")))
		info, err := os.Stat(filepath.Join(workdir, "src", "sub", "f"))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0755), info.Mode().Perm())
	})

	t.Run("ls fails on a missing path", func(t *testing.T) {
		t.Skip("u-root's ls reports a missing path but exits 0")
		workdir := guest(t)
		require.NoError(t, Run(t.Context(), New("ls does-not-exist 2>/dev/null || echo fallback > out")))
		require.Equal(t, "fallback\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("pipes a builtin into a module command", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, Run(t.Context(), New("echo hello | tr a-z A-Z > out")))
		require.Equal(t, "HELLO\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("substitutes the output of module commands", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, Run(t.Context(), New(`n=$(printf abc | wc -c); echo $((n + 1)) > out`)))
		require.Equal(t, "4\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("reports a module command's exit code", func(t *testing.T) {
		native(t)
		err := Run(t.Context(), New("env sh -c 'exit 3'"))
		status, ok := errors.AsType[interp.ExitStatus](err)
		require.True(t, ok, err)
		require.Equal(t, interp.ExitStatus(3), status)
	})

	t.Run("a failing module command reaches ||", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, Run(t.Context(), New("grep x does-not-exist 2>/dev/null || echo fallback > out")))
		require.Equal(t, "fallback\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("sends the command's environment and exported variables to the module", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, Run(t.Context(), New(`export B=2; env sh -c 'echo "$A $B"' > out`).EnvironFrom("A=1")))
		require.Equal(t, "1 2\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("runs module commands in the directory relative to the working directory", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, os.Mkdir(filepath.Join(workdir, "sub"), 0700))
		require.NoError(t, Run(t.Context(), New("env sh -c 'pwd -P' > ../out").Directory("sub")))
		expected, err := filepath.EvalSymlinks(filepath.Join(workdir, "sub"))
		require.NoError(t, err)
		require.Equal(t, expected+"\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("cd moves later module commands", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, os.Mkdir(filepath.Join(workdir, "sub"), 0700))
		require.NoError(t, Run(t.Context(), New("cd sub && touch made")))
		require.FileExists(t, filepath.Join(workdir, "sub", "made"))
	})

	t.Run("bash -c runs in the guest with its name and parameters", func(t *testing.T) {
		workdir := guest(t)
		require.NoError(t, Run(t.Context(), New(`bash -c 'echo "$0 $1 $#" > out' name arg`)))
		require.Equal(t, "name arg 1\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("sh applies its set options", func(t *testing.T) {
		workdir := guest(t)
		err := Run(t.Context(), New("sh -e -c 'false; echo unreachable > out'"))
		status, ok := errors.AsType[interp.ExitStatus](err)
		require.True(t, ok, err)
		require.Equal(t, interp.ExitStatus(1), status)
		require.NoFileExists(t, filepath.Join(workdir, "out"))
	})

	t.Run("a script's exit status is the command's", func(t *testing.T) {
		workdir := guest(t)
		require.NoError(t, Run(t.Context(), New("sh -c 'exit 4'; echo $? > out")))
		require.Equal(t, "4\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("a shebang script runs in the guest", func(t *testing.T) {
		workdir := guest(t)
		require.NoError(t, os.WriteFile(filepath.Join(workdir, "run.sh"), []byte("#!/usr/bin/env bash\necho \"$0 $1\" > out\n"), 0600))
		require.NoError(t, Run(t.Context(), New("./run.sh arg")))
		require.Equal(t, "./run.sh arg\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("sh reads its script from stdin", func(t *testing.T) {
		workdir := guest(t)
		require.NoError(t, Run(t.Context(), New("echo 'echo piped > out' | sh")))
		require.Equal(t, "piped\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("a script's commands reach the module", func(t *testing.T) {
		workdir := native(t)
		require.NoError(t, Run(t.Context(), New(`sh -c 'n=$(printf abc | wc -c); echo $((n + 1))' > out`)))
		require.Equal(t, "4\n", read(t, filepath.Join(workdir, "out")))
	})

	t.Run("a script the guest cannot read goes to the module", func(t *testing.T) {
		guest(t)
		err := Run(t.Context(), New("sh /does/not/exist"))
		_, ok := errors.AsType[interp.ExitStatus](err)
		require.False(t, ok, err)
	})

	t.Run("unsupported shell options go to the module", func(t *testing.T) {
		guest(t)
		err := Run(t.Context(), New("bash --norc -c 'true'"))
		_, ok := errors.AsType[interp.ExitStatus](err)
		require.False(t, ok, err)
	})

	t.Run("invalid syntax", func(t *testing.T) {
		native(t)
		require.Error(t, Run(t.Context(), New("if then")))
	})

	t.Run("cancellation stops the module command", func(t *testing.T) {
		native(t)
		ctx, done := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer done()
		ts := time.Now()
		require.Error(t, Run(ctx, New("sleep 30")))
		require.Less(t, time.Since(ts), 5*time.Second)
	})

	t.Run("wasip1 guest", func(t *testing.T) {
		if testing.Short() {
			t.Skip("builds and runs a wasip1 guest")
		}

		wasm := interptest.Build(t, "./runtime/x/wasi/egbash/.fixtures/guest")
		for _, mode := range []string{"pipeline", "substitution", "devnull", "exit", "script"} {
			t.Run(mode, func(t *testing.T) {
				require.NoError(t, interptest.Run(t, wasm, serve(t), time.Minute, "SHELL_MODE="+mode, eg.EnvComputeWorkingDirectory+"="+eg.DefaultRuntimeDirectory("work")))
			})
		}

		// the runtime directory has no module socket, so the coreutils only succeed if they never leave the guest.
		t.Run("coreutils", func(t *testing.T) {
			// unix socket paths are length limited, t.TempDir can exceed it.
			rundir, err := os.MkdirTemp("", "shell")
			require.NoError(t, err)
			t.Cleanup(func() { os.RemoveAll(rundir) })
			require.NoError(t, os.Mkdir(filepath.Join(rundir, "work"), 0700))
			require.NoError(t, interptest.Run(t, wasm, rundir, time.Minute, "SHELL_MODE=coreutils", eg.EnvComputeWorkingDirectory+"="+eg.DefaultRuntimeDirectory("work")))
		})
	})
}
