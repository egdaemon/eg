package fficoreutils

import (
	"bytes"
	"context"
	"embed"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestReactor(t *testing.T) {
	// the fixture exports the reactor's interface for a few utilities, so these run without a coreutils build.
	module := filepath.Join(t.TempDir(), "reactor.wasm")
	build := exec.Command("go", "build", "-buildmode=c-shared", "-o", module, "./.fixtures/reactor")
	build.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("unable to build the wasip1 reactor: %v\n%s", err, out)
	}

	// load creates a reactor for path, with the guest's empty root and dir mounted at /work.
	load := func(t *testing.T, path string, dir string) *Reactor {
		ctx := context.Background()
		rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
		t.Cleanup(func() { rt.Close(ctx) })
		wasi_snapshot_preview1.MustInstantiate(ctx, rt)

		r, err := Load(ctx, rt, path, wazero.NewModuleConfig().WithFSConfig(wazero.NewFSConfig().WithFSMount(embed.FS{}, "/").WithDirMount(dir, "/work")), Streams{})
		require.NoError(t, err)
		t.Cleanup(func() { r.Close(ctx) })
		return r
	}

	call := func(ctx context.Context, r *Reactor, stdin io.Reader, dir string, name string, args ...string) (uint32, string, error) {
		var (
			out bytes.Buffer
		)

		code, err := r.Call(WithStreams(ctx, Streams{Stdin: stdin, Stdout: &out, Stderr: &out}), dir, name, args)
		return code, out.String(), err
	}

	t.Run("reuses one instance for calls in turn", func(t *testing.T) {
		r := load(t, module, t.TempDir())
		for _, word := range []string{"one", "two", "three"} {
			code, out, err := call(t.Context(), r, nil, "", "echo", word)
			require.NoError(t, err)
			require.Zero(t, code)
			require.Equal(t, word+"\n", out)
		}
		require.Len(t, r.idle, 1)
	})

	t.Run("returns a utility's exit code and keeps its instance", func(t *testing.T) {
		r := load(t, module, t.TempDir())
		code, out, err := call(t.Context(), r, nil, "/work", "cat", "missing")
		require.NoError(t, err)
		require.Equal(t, uint32(1), code)
		require.Contains(t, out, "missing")
		require.Len(t, r.idle, 1)
	})

	t.Run("runs the calls of a pipeline at the same time", func(t *testing.T) {
		r := load(t, module, t.TempDir())
		ctx, done := context.WithTimeout(t.Context(), 10*time.Second)
		defer done()

		pr, pw := io.Pipe()
		upper := make(chan string, 1)
		go func() {
			_, out, err := call(ctx, r, pr, "", "upper")
			require.NoError(t, err)
			upper <- out
		}()

		// upper reads what echo writes, so with one instance for both neither would finish.
		code, err := r.Call(WithStreams(ctx, Streams{Stdout: pw}), "", "echo", []string{"piped"})
		require.NoError(t, err)
		require.Zero(t, code)
		require.NoError(t, pw.Close())
		require.Equal(t, "PIPED\n", <-upper)
		require.Len(t, r.idle, 2)
	})

	t.Run("resolves relative paths against the call's directory", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "f.txt"), []byte("relative\n"), 0600))
		r := load(t, module, dir)

		_, out, err := call(t.Context(), r, nil, "/work/sub", "cat", "f.txt")
		require.NoError(t, err)
		require.Equal(t, "relative\n", out)

		// the instance is shared, so the next call's directory replaces the previous one.
		_, out, err = call(t.Context(), r, nil, "/work", "where")
		require.NoError(t, err)
		require.Equal(t, "/work\n", out)

		_, out, err = call(t.Context(), r, nil, "", "where")
		require.NoError(t, err)
		require.Equal(t, "/\n", out)
	})

	t.Run("fails a call to a directory outside of the mounts", func(t *testing.T) {
		r := load(t, module, t.TempDir())
		_, _, err := call(t.Context(), r, nil, "/missing", "where")
		require.ErrorContains(t, err, "unable to change to /missing")
	})

	t.Run("stops a call at its deadline and replaces its instance", func(t *testing.T) {
		r := load(t, module, t.TempDir())
		ctx, done := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer done()

		ts := time.Now()
		_, err := r.Call(WithStreams(ctx, Streams{Stdout: io.Discard}), "", "yes", nil)
		require.Error(t, err)
		require.Less(t, time.Since(ts), 5*time.Second)
		require.Empty(t, r.idle)

		code, out, err := call(t.Context(), r, nil, "", "echo", "after")
		require.NoError(t, err)
		require.Zero(t, code)
		require.Equal(t, "after\n", out)
	})

	t.Run("replaces an instance a utility kills", func(t *testing.T) {
		r := load(t, module, t.TempDir())
		_, _, err := call(t.Context(), r, nil, "", "trap")
		require.Error(t, err)
		require.Empty(t, r.idle)

		_, out, err := call(t.Context(), r, nil, "", "echo", "after")
		require.NoError(t, err)
		require.Equal(t, "after\n", out)
	})

	t.Run("reports a utility the module does not provide", func(t *testing.T) {
		r := load(t, module, t.TempDir())
		_, _, err := call(t.Context(), r, nil, "", "sort")
		require.ErrorContains(t, err, `coreutils does not provide "sort"`)
	})

	t.Run("without the module provides nothing", func(t *testing.T) {
		r := load(t, filepath.Join(t.TempDir(), "missing.wasi"), t.TempDir())
		_, _, err := call(t.Context(), r, nil, "", "echo", "x")
		require.ErrorContains(t, err, "coreutils is not installed")
	})

	t.Run("refuses calls once closed", func(t *testing.T) {
		r := load(t, module, t.TempDir())
		_, _, err := call(t.Context(), r, nil, "", "echo", "x")
		require.NoError(t, err)
		require.NoError(t, r.Close(t.Context()))
		require.Empty(t, r.idle)

		_, _, err = call(t.Context(), r, nil, "", "echo", "x")
		require.ErrorContains(t, err, "coreutils is closed")
	})
}
