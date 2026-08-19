package gitx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsRepository(t *testing.T) {
	t.Run("true for a git repository", func(t *testing.T) {
		dir := t.TempDir()
		initRepo(t, dir, map[string]string{"main.go": "package main"})
		require.True(t, IsRepository(dir))
	})

	t.Run("false for a plain directory", func(t *testing.T) {
		require.False(t, IsRepository(t.TempDir()))
	})

	t.Run("true for a subdirectory inside a git repository", func(t *testing.T) {
		dir := t.TempDir()
		initRepo(t, dir, map[string]string{"main.go": "package main"})
		subdir := filepath.Join(dir, "pkg", "main")
		require.NoError(t, os.MkdirAll(subdir, 0755))
		require.True(t, IsRepository(subdir))
	})

	t.Run("true for a deeply nested subdirectory inside a git repository", func(t *testing.T) {
		dir := t.TempDir()
		initRepo(t, dir, map[string]string{"main.go": "package main"})
		deep := filepath.Join(dir, "a", "b", "c", "d", "e")
		require.NoError(t, os.MkdirAll(deep, 0755))
		require.True(t, IsRepository(deep))
	})

	t.Run("false for a sibling of a git repository", func(t *testing.T) {
		repoDir := t.TempDir()
		initRepo(t, repoDir, map[string]string{"main.go": "package main"})
		sibling := filepath.Join(filepath.Dir(repoDir), "other")
		require.NoError(t, os.MkdirAll(sibling, 0755))
		require.False(t, IsRepository(sibling))
	})

	t.Run("false for an ancestor that is not itself a git repository", func(t *testing.T) {
		repoDir := t.TempDir()
		initRepo(t, repoDir, map[string]string{"main.go": "package main"})
		// fsx.LocateWithin walks upward only; .git lives below the ancestor,
		// so a non-repo ancestor correctly returns false.
		grandparent := filepath.Dir(filepath.Dir(repoDir))
		require.False(t, IsRepository(grandparent))
	})

	t.Run("false for a directory containing a git repository but not inside it", func(t *testing.T) {
		repoDir := t.TempDir()
		initRepo(t, repoDir, map[string]string{"main.go": "package main"})
		parent := filepath.Dir(repoDir)
		require.False(t, IsRepository(parent))
	})
}
