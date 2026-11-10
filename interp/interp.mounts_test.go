package interp

import (
	"archive/tar"
	"bytes"
	"context"
	"embed"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	experimentalsys "github.com/tetratelabs/wazero/experimental/sys"
	"github.com/tetratelabs/wazero/experimental/sysfs"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestMounts(t *testing.T) {
	wasm := interptest.Build(t, "./interp/.xfixtures/mounts")

	workspace := func(t *testing.T) workspaces.Context {
		return workspaces.Context{
			Root:         t.TempDir(),
			RuntimeDir:   t.TempDir(),
			CacheDir:     t.TempDir(),
			WorkspaceDir: t.TempDir(),
			WorkingDir:   t.TempDir(),
		}
	}

	empty := &sysfs.AdaptFS{FS: embed.FS{}}

	// run performs the operation in a guest with the workspace mounted, returning an error when it fails.
	run := func(t *testing.T, root experimentalsys.FS, ws workspaces.Context, args ...string) error {
		ctx, done := context.WithTimeout(context.Background(), time.Minute)
		defer done()

		rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
		defer rt.Close(context.Background())

		_, err := wasi_snapshot_preview1.NewBuilder(rt).Instantiate(ctx)
		require.NoError(t, err)

		bin, err := os.ReadFile(wasm)
		require.NoError(t, err)

		_, err = rt.InstantiateWithConfig(ctx, bin, wazero.NewModuleConfig().
			WithFSConfig(mounts(ws, root)).
			WithArgs(append([]string{"guest"}, args...)...).
			WithStderr(os.Stderr))
		return err
	}

	t.Run("lists the root", func(t *testing.T) {
		require.NoError(t, run(t, empty, workspace(t), "readdir", "/"))
	})

	t.Run("creates nested directories in the working directory", func(t *testing.T) {
		ws := workspace(t)
		require.NoError(t, run(t, empty, ws, "mkdirall", eg.DefaultWorkingDirectory("a", "b")))
		require.DirExists(t, filepath.Join(ws.WorkingDir, "a", "b"))
	})

	t.Run("renames within the working directory", func(t *testing.T) {
		ws := workspace(t)
		require.NoError(t, os.WriteFile(filepath.Join(ws.WorkingDir, "from"), []byte("hello"), 0600))
		require.NoError(t, run(t, empty, ws, "rename", eg.DefaultWorkingDirectory("from"), eg.DefaultWorkingDirectory("to")))
		require.FileExists(t, filepath.Join(ws.WorkingDir, "to"))
		require.NoFileExists(t, filepath.Join(ws.WorkingDir, "from"))
	})

	t.Run("writes to the host path of a mount", func(t *testing.T) {
		ws := workspace(t)
		require.NoError(t, run(t, empty, ws, "write", filepath.Join(ws.CacheDir, "file")))
		require.FileExists(t, filepath.Join(ws.CacheDir, "file"))
	})

	t.Run("host files outside of the mounts are missing", func(t *testing.T) {
		require.Error(t, run(t, empty, workspace(t), "read", "/etc/hosts"))
	})

	t.Run("refuses to write to the root", func(t *testing.T) {
		require.Error(t, run(t, empty, workspace(t), "write", "/escape"))
		require.NoFileExists(t, "/escape")
	})

	t.Run("refuses to create directories in the root", func(t *testing.T) {
		require.Error(t, run(t, empty, workspace(t), "mkdirall", "/escape/nested"))
		require.NoDirExists(t, "/escape")
	})

	t.Run("an egiso root shows its image beneath the workspace mounts", func(t *testing.T) {
		var (
			buf bytes.Buffer
			out bytes.Buffer
		)

		tw := tar.NewWriter(&buf)
		require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "etc/", Mode: 0755}))
		require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644, Size: 4}))
		_, err := tw.Write([]byte("egd\n"))
		require.NoError(t, err)
		require.NoError(t, tw.Close())

		toc, err := egiso.Build(io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len())), &out)
		require.NoError(t, err)
		archive, err := egiso.New(io.NewSectionReader(bytes.NewReader(out.Bytes()), 0, int64(out.Len())), toc)
		require.NoError(t, err)
		upperdir := t.TempDir()
		upper, err := os.OpenRoot(upperdir)
		require.NoError(t, err)
		defer upper.Close()

		root := egiso.NewSysFS(egiso.NewUpper(egiso.Tree{}.Mount(".", archive, egiso.NewOwners(), egiso.NewOwners()), upper), egiso.Default, egiso.Default)
		ws := workspace(t)
		require.NoError(t, run(t, root, ws, "read", "/etc/hostname"))
		require.NoError(t, run(t, root, ws, "write", "/etc/motd"))
		require.FileExists(t, filepath.Join(upperdir, "etc", "motd"))

		require.NoError(t, run(t, root, ws, "write", eg.DefaultWorkingDirectory("file")))
		require.FileExists(t, filepath.Join(ws.WorkingDir, "file"))
		require.NoFileExists(t, filepath.Join(upperdir, strings.TrimPrefix(eg.DefaultWorkingDirectory("file"), "/")))
	})
}
