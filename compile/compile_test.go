package compile_test

import (
	"crypto/md5"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
)

func TestFromTranspiled(t *testing.T) {
	t.Run("should compile example 1", func(t *testing.T) {
		ctx := t.Context()
		srcdir := t.TempDir()
		tmpdir := t.TempDir()
		ws, err := workspaces.New(ctx, md5.New(), tmpdir, tmpdir, "")
		require.NoError(t, err)

		require.NoError(t, fsx.CloneTree(ctx, srcdir, filepath.Join("example.1", eg.DefaultModuleDirectory()), os.DirFS(testx.Fixture())))

		roots, err := transpile.Autodetect(transpile.New(srcdir, ws)).Run(ctx)
		require.NoError(t, err)

		err = compile.EnsureRequiredPackages(ctx, filepath.Join(ws.Root, ws.TransDir))
		require.NoError(t, err)
		modules, err := compile.FromTranspiled(ctx, ws, roots...)
		require.NoError(t, err)
		require.Len(t, modules, 1)
		require.False(t, modules[0].Generated)
		require.Equal(t, filepath.Join(tmpdir, ws.BuildDir, "main.wasm"), modules[0].Path)
	})

	t.Run("should transform nested modules", func(t *testing.T) {
		ctx := t.Context()
		srcdir := t.TempDir()
		tmpdir := t.TempDir()
		ws, err := workspaces.New(ctx, md5.New(), tmpdir, tmpdir, "")
		require.NoError(t, err)

		require.NoError(t, fsx.CloneTree(ctx, srcdir, filepath.Join("example.2", eg.DefaultModuleDirectory()), os.DirFS(testx.Fixture())))

		roots, err := transpile.Autodetect(transpile.New(srcdir, ws)).Run(ctx)
		require.NoError(t, err)
		err = compile.EnsureRequiredPackages(ctx, filepath.Join(ws.Root, ws.TransDir))
		require.NoError(t, err)

		modules, err := compile.FromTranspiled(ctx, ws, roots...)
		require.NoError(t, err)
		require.Len(t, modules, 1)
		require.False(t, modules[0].Generated)
		require.Equal(t, filepath.Join(tmpdir, ws.BuildDir, "main.wasm"), modules[0].Path)
		require.Equal(t, "6d5e29ce-6e99-d52f-f8c6-4ab44bee50b1", testx.ReadMD5(filepath.Join(tmpdir, ws.TransDir, "m1", "m1.go")), testx.ReadString(filepath.Join(tmpdir, ws.TransDir, "m1", "m1.go")))
		require.Equal(t, "8d6b4444-b948-e467-8435-24d7c4fea235", testx.ReadMD5(filepath.Join(tmpdir, ws.TransDir, "m1", "m2", "m2.go")))
	})
}

func TestWasixWarmCache(t *testing.T) {
	t.Run("should compile example 1 and warm the provided cache", func(t *testing.T) {
		ctx := t.Context()
		srcdir := t.TempDir()
		tmpdir := t.TempDir()
		ws, err := workspaces.New(ctx, md5.New(), tmpdir, tmpdir, "")
		require.NoError(t, err)

		require.NoError(t, fsx.CloneTree(ctx, srcdir, filepath.Join("example.1", eg.DefaultModuleDirectory()), os.DirFS(testx.Fixture())))

		wazcache, err := os.MkdirTemp(tmpdir, "wazcache")
		require.NoError(t, err)

		roots, err := transpile.Autodetect(transpile.New(srcdir, ws)).Run(ctx)
		require.NoError(t, err)
		err = compile.EnsureRequiredPackages(ctx, filepath.Join(ws.Root, ws.TransDir))
		require.NoError(t, err)

		_, err = compile.FromTranspiled(ctx, ws, roots...)
		require.NoError(t, err)

		cache, err := wazero.NewCompilationCacheWithDir(wazcache)
		require.NoError(t, err)
		defer cache.Close(ctx)
		require.NoError(t, wasix.WarmCacheFromDirectoryTree(ctx, filepath.Join(ws.Root, ws.BuildDir), cache))

		cached, err := fs.ReadDir(os.DirFS(wazcache), ".")
		require.NoError(t, err)
		require.Len(t, cached, 1)

		entries, err := fs.ReadDir(os.DirFS(wazcache), cached[0].Name())
		require.NoError(t, err)
		require.Len(t, entries, 3)
	})
}
