package execx

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestLookPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "tool")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))
	plain := filepath.Join(dir, "plain")
	require.NoError(t, os.WriteFile(plain, []byte("data"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755))

	t.Run("absolute path", func(t *testing.T) {
		p, err := LookPath(bin)
		require.NoError(t, err)
		require.Equal(t, bin, p)
	})

	t.Run("absolute path not executable", func(t *testing.T) {
		_, err := LookPath(plain)
		require.ErrorIs(t, err, fs.ErrPermission)
	})

	t.Run("absolute path is a directory", func(t *testing.T) {
		_, err := LookPath(filepath.Join(dir, "subdir"))
		require.ErrorIs(t, err, fs.ErrPermission)
	})

	t.Run("absolute path missing", func(t *testing.T) {
		_, err := LookPath(filepath.Join(dir, "missing"))
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("dot prefixed paths skip the PATH search", func(t *testing.T) {
		t.Chdir(dir)
		p, err := LookPath("./tool")
		require.NoError(t, err)
		require.Equal(t, "./tool", p)

		p, err = LookPath("../" + filepath.Base(dir) + "/tool")
		require.NoError(t, err)
		require.Equal(t, "../"+filepath.Base(dir)+"/tool", p)
	})

	t.Run("found on PATH", func(t *testing.T) {
		testx.Tempenvvar("PATH", dir, func() {
			p, err := LookPath("tool")
			require.NoError(t, err)
			require.Equal(t, bin, p)
		})
	})

	t.Run("skips non executables on PATH", func(t *testing.T) {
		testx.Tempenvvar("PATH", dir, func() {
			_, err := LookPath("plain")
			require.ErrorIs(t, err, exec.ErrNotFound)
		})
	})

	t.Run("not found on PATH", func(t *testing.T) {
		testx.Tempenvvar("PATH", dir, func() {
			_, err := LookPath("missing")
			require.ErrorIs(t, err, exec.ErrNotFound)
		})
	})

	t.Run("relative PATH entry is rejected", func(t *testing.T) {
		t.Chdir(filepath.Dir(dir))
		testx.Tempenvvar("PATH", filepath.Base(dir), func() {
			p, err := LookPath("tool")
			require.ErrorIs(t, err, exec.ErrDot)
			require.Equal(t, filepath.Join(filepath.Base(dir), "tool"), p)
		})
	})
}
