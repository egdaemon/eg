package gitx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChanges(t *testing.T) {
	// a repository with a.txt and b.txt committed, and a clone of it.
	setup := func(t *testing.T) (repo string, clone string) {
		repo = t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n", "b.txt": "one\n"})

		clone = filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))

		return repo, clone
	}

	t.Run("is empty without changes", func(t *testing.T) {
		repo, clone := setup(t)
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)
		require.Empty(t, patch)
	})

	t.Run("includes modified and new files", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "new.txt"), []byte("new\n"), 0644))

		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)
		require.Contains(t, string(patch), "+++ b/a.txt")
		require.Contains(t, string(patch), "+two")
		require.Contains(t, string(patch), "+++ b/new.txt")
		require.Contains(t, string(patch), "+new")
	})

	t.Run("leaves the repository alone", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))

		_, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		raw, err := os.ReadFile(filepath.Join(repo, "a.txt"))
		require.NoError(t, err)
		require.Equal(t, "one\n", string(raw))
	})

	t.Run("is empty for a clone directory that is the repository", func(t *testing.T) {
		repo, _ := setup(t)
		link := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(repo, link))
		require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("two\n"), 0644))

		patch, err := Changes(t.Context(), link, repo)
		require.NoError(t, err)
		require.Empty(t, patch)
	})

	t.Run("is empty for a clone directory that is not a repository", func(t *testing.T) {
		repo, _ := setup(t)
		patch, err := Changes(t.Context(), t.TempDir(), repo)
		require.NoError(t, err)
		require.Empty(t, patch)
	})
}
