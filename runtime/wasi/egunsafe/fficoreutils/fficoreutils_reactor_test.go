package fficoreutils_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiwasinet"
	"github.com/egdaemon/eg/interp/runtime/wasi/wasimod"
	"github.com/egdaemon/wasinet/wasinet/wnetruntime"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// TestReactor runs commands through the egcoreutils reactor at EG_COREUTILS_WASM, see cmd/eg/README.wasi.md
// for building it.
func TestReactor(t *testing.T) {
	reactor := os.Getenv("EG_COREUTILS_WASM")
	if reactor == "" {
		t.Skip("EG_COREUTILS_WASM is not set")
	}

	guest := filepath.Join(t.TempDir(), "guest.wasm")
	cmd := exec.Command("go", "build", "-o", guest, "./.xfixtures/guest")
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("unable to build the guest: %v\n%s", err, b)
	}

	mods := t.TempDir()
	require.NoError(t, os.Symlink(reactor, filepath.Join(mods, "egcoreutils.wasi")))

	ctx := t.Context()
	runtimedir := t.TempDir()
	rt := wazero.NewRuntime(ctx)
	t.Cleanup(func() { rt.Close(ctx) })
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	_, err := ffiwasinet.Wazero(
		rt,
		wnetruntime.OptionFSPrefixes(wnetruntime.FSPrefix{Host: runtimedir, Guest: _eg.DefaultRuntimeDirectory()}),
	).Instantiate(ctx)
	require.NoError(t, err)

	var stderr bytes.Buffer
	mcfg := wazero.NewModuleConfig().
		WithFSConfig(wazero.NewFSConfig().WithDirMount(runtimedir, _eg.DefaultRuntimeDirectory())).
		WithStderr(&stderr).
		WithOsyield(runtime.Gosched).
		WithSysNanosleep().
		WithSysNanotime().
		WithSysWalltime()
	_, err = wasimod.Load(ctx, rt, mods, mcfg)
	require.NoError(t, err)

	bin, err := os.ReadFile(guest)
	require.NoError(t, err)
	compiled, err := rt.CompileModule(ctx, bin)
	require.NoError(t, err)

	run := func(t *testing.T, args ...string) string {
		var stdout bytes.Buffer
		m, err := rt.InstantiateModule(ctx, compiled, mcfg.WithName("").WithStdout(&stdout).WithArgs(append([]string{"guest"}, args...)...))
		require.NoError(t, err, stderr.String())
		m.Close(ctx)
		// the reactor reports what it can not hand to a command on its own stderr.
		if stderr.Len() > 0 {
			t.Log(stderr.String())
			stderr.Reset()
		}
		return stdout.String()
	}

	t.Run("echo writes to stdout", func(t *testing.T) {
		require.Equal(t, "\"hi\" \"\" OK\n", run(t, "echo"))
	})

	t.Run("tee copies stdin to stdout and the file", func(t *testing.T) {
		require.Equal(t, "\"abc\" \"\" OK\n", run(t, "tee", _eg.DefaultRuntimeDirectory("tee.out")))
		b, err := os.ReadFile(filepath.Join(runtimedir, "tee.out"))
		require.NoError(t, err)
		require.Equal(t, "abc", string(b))
	})

	t.Run("concurrent commands keep their own environment", func(t *testing.T) {
		require.Equal(t, "\"-e a\\tb\\n\" \"\" OK\n\"a\\tb\\n\" \"\" OK\n", run(t, "concurrent"))
	})

	t.Run("a utility the reactor does not run is not found", func(t *testing.T) {
		require.Equal(t, "\"\" \"\" NotFound\n", run(t, "ls"))
	})

	t.Run("the processes are removed", func(t *testing.T) {
		procs, err := os.ReadDir(filepath.Join(runtimedir, "proc"))
		require.NoError(t, err)
		require.Empty(t, procs)
	})
}
