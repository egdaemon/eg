package eggit

import (
	"path/filepath"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/stretchr/testify/require"
)

func TestAutoCached(t *testing.T) {
	t.Run("rooted in the git cache", func(t *testing.T) {
		cache := t.TempDir()
		t.Setenv(_eg.EnvComputeCacheDirectory, cache)

		uri := "https://example.com/repo.git"
		require.Equal(t, filepath.Join(cache, "git", md5x.String(uri)+".git"), AutoCached(uri))
	})

	t.Run("stable for a uri", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeCacheDirectory, t.TempDir())
		require.Equal(t, AutoCached("https://example.com/a.git"), AutoCached("https://example.com/a.git"))
	})

	t.Run("distinct per uri", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeCacheDirectory, t.TempDir())
		require.NotEqual(t, AutoCached("https://example.com/a.git"), AutoCached("https://example.com/b.git"))
	})
}

func TestCachedClone(t *testing.T) {
	t.Run("blank uri", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeCacheDirectory, t.TempDir())
		err := CachedClone(" ", "origin", "main").Op(t.Context(), nil)
		require.ErrorContains(t, err, "url not specified")
	})

	t.Run("blank remote", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeCacheDirectory, t.TempDir())
		err := CachedClone("https://example.com/repo.git", "\t", "main").Op(t.Context(), nil)
		require.ErrorContains(t, err, "remote not specified")
	})

	t.Run("host call unavailable outside of wasi", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeCacheDirectory, t.TempDir())
		err := CachedClone("https://example.com/repo.git", "origin", "main").Op(t.Context(), nil)
		require.Error(t, err)
	})
}
