package egbash

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/egdaemon/sh/v3/interp"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	if os.Getenv("EG_TEST_WASIP1") == "" {
		t.Skip("builds and runs a wasip1 guest, set EG_TEST_WASIP1=1 to enable")
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

	t.Run("cd moves later commands", func(t *testing.T) {
		workdir := guest(t)
		require.NoError(t, os.Mkdir(filepath.Join(workdir, "sub"), 0700))
		require.NoError(t, Run(t.Context(), New("cd sub && touch made")))
		require.FileExists(t, filepath.Join(workdir, "sub", "made"))
	})

	t.Run("refuses a command neither the guest nor its coreutils provide", func(t *testing.T) {
		guest(t)
		err := Run(t.Context(), New("grep x does-not-exist"))
		_, ok := errors.AsType[interp.ExitStatus](err)
		require.False(t, ok, err)
		require.ErrorContains(t, err, "coreutils does not provide grep")
	})

	t.Run("invalid syntax", func(t *testing.T) {
		guest(t)
		require.Error(t, Run(t.Context(), New("if then")))
	})

	t.Run("wasip1 guest", func(t *testing.T) {
		wasm := interptest.Build(t, "./runtime/x/wasi/egbash/.fixtures/guest")
		// the commands the guest's coreutils lack run in the egcoreutils reactor, which the runtime loads from the
		// wasi module directory.
		for _, mode := range []string{"pipeline", "substitution", "devnull", "exit", "environ", "coreutils"} {
			t.Run(mode, func(t *testing.T) {
				// unix socket paths are length limited, t.TempDir can exceed it.
				rundir, err := os.MkdirTemp("", "shell")
				require.NoError(t, err)
				t.Cleanup(func() { os.RemoveAll(rundir) })
				require.NoError(t, os.Mkdir(filepath.Join(rundir, "work"), 0700))
				require.NoError(t, interptest.Run(t, wasm, rundir, time.Minute, "SHELL_MODE="+mode, eg.EnvComputeWorkingDirectory+"="+eg.DefaultRuntimeDirectory("work")))
			})
		}
	})
}
