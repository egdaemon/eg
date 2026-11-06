package fficoreutils_test

import (
	"bytes"
	"context"
	"embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg/interp/runtime/wasi/fficoreutils"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// EG_TEST_COREUTILS_WASM must point at a coreutils reactor module (wasm32-wasip1 with feat_wasm_reactor).
func TestRunFromGuest(t *testing.T) {
	reactor := os.Getenv("EG_TEST_COREUTILS_WASM")
	if reactor == "" {
		t.Skip("EG_TEST_COREUTILS_WASM not set")
	}

	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("file contents\n"), 0600))

	guestpath := filepath.Join(t.TempDir(), "guest.wasm")
	build := exec.Command("go", "build", "-o", guestpath, "./.xfixtures/guest")
	build.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	out, err := build.CombinedOutput()
	require.NoError(t, err, string(out))

	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
	defer rt.Close(ctx)

	_, err = wasi_snapshot_preview1.NewBuilder(rt).Instantiate(ctx)
	require.NoError(t, err)

	raw, err := os.ReadFile(reactor)
	require.NoError(t, err)
	compiled, err := rt.CompileModule(ctx, raw)
	require.NoError(t, err)

	// the reactor has no stdio of its own, everything below is provided by the guest.
	host := fficoreutils.New(
		rt, compiled,
		wazero.NewModuleConfig().WithFSConfig(wazero.NewFSConfig().WithFSMount(embed.FS{}, "/").WithDirMount(dir, "/work")),
		fficoreutils.Streams{},
	)
	defer host.Close(ctx)

	_, err = rt.NewHostModuleBuilder("env").
		NewFunctionBuilder().WithFunc(host.Run()).
		Export("github.com/egdaemon/eg/runtime/wasi/runtime/fficoreutils.Run").
		Instantiate(ctx)
	require.NoError(t, err)

	guest, err := os.ReadFile(guestpath)
	require.NoError(t, err)
	gcompiled, err := rt.CompileModule(ctx, guest)
	require.NoError(t, err)

	// the embedder attaches the guest's stdio to the context it runs the guest with.
	var gstdout, gstderr bytes.Buffer
	gctx := fficoreutils.WithStreams(ctx, fficoreutils.Streams{Stdin: strings.NewReader("guest stdin\n"), Stdout: &gstdout, Stderr: &gstderr})
	_, err = rt.InstantiateModule(gctx, gcompiled, wazero.NewModuleConfig().WithName("guest").WithStdout(&gstdout).WithStderr(&gstderr))
	require.NoError(t, err, gstderr.String())

	// output lands on the guest's own stdout/stderr.
	require.Equal(t, "hello coreutils\nfile contents\na.txt\nguest stdin\n", gstdout.String())
	require.Contains(t, gstderr.String(), "missing")
}

func TestCallStreamsConcurrent(t *testing.T) {
	reactor := os.Getenv("EG_TEST_COREUTILS_WASM")
	if reactor == "" {
		t.Skip("EG_TEST_COREUTILS_WASM not set")
	}

	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("file contents\n"), 0600))

	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())
	defer rt.Close(ctx)

	_, err := wasi_snapshot_preview1.NewBuilder(rt).Instantiate(ctx)
	require.NoError(t, err)

	raw, err := os.ReadFile(reactor)
	require.NoError(t, err)
	compiled, err := rt.CompileModule(ctx, raw)
	require.NoError(t, err)

	r := fficoreutils.New(
		rt, compiled,
		wazero.NewModuleConfig().WithFSConfig(wazero.NewFSConfig().WithFSMount(embed.FS{}, "/").WithDirMount(dir, "/work")),
		fficoreutils.Streams{},
	)
	defer r.Close(ctx)

	// every call has its own stdout/stderr, nothing is shared between them.
	const n = 8
	var wg sync.WaitGroup
	for range n {
		wg.Add(3)
		go func() {
			defer wg.Done()
			var out, errout bytes.Buffer
			cctx := fficoreutils.WithStreams(ctx, fficoreutils.Streams{Stdout: &out, Stderr: &errout})
			code, err := r.Call(cctx, "", "ls", []string{"--help"})
			require.NoError(t, err)
			require.Zero(t, code, errout.String())
			require.Contains(t, out.String(), "Usage: ls")
			require.NotContains(t, out.String(), "Usage: cat")
			require.Empty(t, errout.String())
		}()
		go func() {
			defer wg.Done()
			var out, errout bytes.Buffer
			cctx := fficoreutils.WithStreams(ctx, fficoreutils.Streams{
				Stdin:  strings.NewReader("from stdin\n"),
				Stdout: &out,
				Stderr: &errout,
			})
			code, err := r.Call(cctx, "", "cat", nil)
			require.NoError(t, err)
			require.Zero(t, code, errout.String())
			require.Equal(t, "from stdin\n", out.String())
		}()
		go func() {
			defer wg.Done()
			// fails: the message goes to this call's stderr, the exit code does not leak.
			var out, errout bytes.Buffer
			cctx := fficoreutils.WithStreams(ctx, fficoreutils.Streams{Stdout: &out, Stderr: &errout})
			code, err := r.Call(cctx, "", "cat", []string{"/work/missing"})
			require.NoError(t, err)
			require.NotZero(t, code)
			require.Empty(t, out.String())
			require.Contains(t, errout.String(), "missing")
		}()
	}
	wg.Wait()
}

// the Go fixture's own sleep is unreliable inside an exported call, so sleeps are tested against the coreutils
// reactor.
func TestCallSleep(t *testing.T) {
	reactor := os.Getenv("EG_TEST_COREUTILS_WASM")
	if reactor == "" {
		t.Skip("EG_TEST_COREUTILS_WASM not set")
	}

	ctx := context.Background()
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	defer rt.Close(ctx)

	_, err := wasi_snapshot_preview1.NewBuilder(rt).Instantiate(ctx)
	require.NoError(t, err)

	r, err := fficoreutils.Load(ctx, rt, reactor, wazero.NewModuleConfig().WithFSConfig(wazero.NewFSConfig().WithFSMount(embed.FS{}, "/")), fficoreutils.Streams{})
	require.NoError(t, err)
	defer r.Close(ctx)

	t.Run("sleeps for the duration", func(t *testing.T) {
		ts := time.Now()
		code, err := r.Call(ctx, "", "sleep", []string{"0.2"})
		require.NoError(t, err)
		require.Zero(t, code)
		require.GreaterOrEqual(t, time.Since(ts), 200*time.Millisecond)
		require.Less(t, time.Since(ts), 5*time.Second)
	})

	t.Run("a deadline ends the sleep", func(t *testing.T) {
		dctx, done := context.WithTimeout(ctx, 300*time.Millisecond)
		defer done()

		ts := time.Now()
		_, err := r.Call(dctx, "", "sleep", []string{"30"})
		require.Error(t, err)
		require.Less(t, time.Since(ts), 5*time.Second)

		code, err := r.Call(ctx, "", "true", nil)
		require.NoError(t, err)
		require.Zero(t, code)
	})
}
