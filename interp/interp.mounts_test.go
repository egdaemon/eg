package interp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestMounts(t *testing.T) {
	wasm := interptest.Build(t, "./interp/.fixtures/mounts")

	workspace := func(t *testing.T) workspaces.Context {
		return workspaces.Context{
			Root:         t.TempDir(),
			RuntimeDir:   t.TempDir(),
			CacheDir:     t.TempDir(),
			WorkspaceDir: t.TempDir(),
			WorkingDir:   t.TempDir(),
		}
	}

	// run performs the operation in a guest with the workspace mounted, returning an error when it fails.
	run := func(t *testing.T, ws workspaces.Context, args ...string) error {
		ctx, done := context.WithTimeout(context.Background(), time.Minute)
		defer done()

		rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
		defer rt.Close(context.Background())

		_, err := wasi_snapshot_preview1.NewBuilder(rt).Instantiate(ctx)
		require.NoError(t, err)

		bin, err := os.ReadFile(wasm)
		require.NoError(t, err)

		_, err = rt.InstantiateWithConfig(ctx, bin, wazero.NewModuleConfig().
			WithFSConfig(mounts(ws)).
			WithArgs(append([]string{"guest"}, args...)...).
			WithStderr(os.Stderr))
		return err
	}

	t.Run("lists the root", func(t *testing.T) {
		require.NoError(t, run(t, workspace(t), "readdir", "/"))
	})

	t.Run("creates nested directories in the working directory", func(t *testing.T) {
		ws := workspace(t)
		require.NoError(t, run(t, ws, "mkdirall", eg.DefaultWorkingDirectory("a", "b")))
		require.DirExists(t, filepath.Join(ws.WorkingDir, "a", "b"))
	})

	t.Run("renames within the working directory", func(t *testing.T) {
		ws := workspace(t)
		require.NoError(t, os.WriteFile(filepath.Join(ws.WorkingDir, "from"), []byte("hello"), 0600))
		require.NoError(t, run(t, ws, "rename", eg.DefaultWorkingDirectory("from"), eg.DefaultWorkingDirectory("to")))
		require.FileExists(t, filepath.Join(ws.WorkingDir, "to"))
		require.NoFileExists(t, filepath.Join(ws.WorkingDir, "from"))
	})

	t.Run("writes to the host path of a mount", func(t *testing.T) {
		ws := workspace(t)
		require.NoError(t, run(t, ws, "write", filepath.Join(ws.CacheDir, "file")))
		require.FileExists(t, filepath.Join(ws.CacheDir, "file"))
	})

	t.Run("host files outside of the mounts are missing", func(t *testing.T) {
		require.Error(t, run(t, workspace(t), "read", "/etc/hosts"))
	})

	t.Run("refuses to write to the root", func(t *testing.T) {
		require.Error(t, run(t, workspace(t), "write", "/escape"))
		require.NoFileExists(t, "/escape")
	})

	t.Run("refuses to create directories in the root", func(t *testing.T) {
		require.Error(t, run(t, workspace(t), "mkdirall", "/escape/nested"))
		require.NoDirExists(t, "/escape")
	})
}
