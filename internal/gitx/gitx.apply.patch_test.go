package gitx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyPatch(t *testing.T) {
	// a repository with a.txt and b.txt committed, and a clone of it.
	setup := func(t *testing.T) (repo string, clone string) {
		repo = t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n", "b.txt": "one\n"})

		clone = filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))

		return repo, clone
	}

	read := func(t *testing.T, path string) string {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(raw)
	}

	t.Run("does nothing without a patch", func(t *testing.T) {
		repo, _ := setup(t)
		require.NoError(t, ApplyPatch(t.Context(), repo, nil))
		require.Equal(t, "one\n", read(t, filepath.Join(repo, "a.txt")))
	})

	t.Run("applies modified and new files", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "new.txt"), []byte("new\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		require.NoError(t, ApplyPatch(t.Context(), repo, patch))
		require.Equal(t, "two\n", read(t, filepath.Join(repo, "a.txt")))
		require.Equal(t, "new\n", read(t, filepath.Join(repo, "new.txt")))
	})

	t.Run("keeps unrelated uncommitted changes", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(repo, "b.txt"), []byte("mine\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		require.NoError(t, ApplyPatch(t.Context(), repo, patch))
		require.Equal(t, "two\n", read(t, filepath.Join(repo, "a.txt")))
		require.Equal(t, "mine\n", read(t, filepath.Join(repo, "b.txt")))
	})

	t.Run("skips changes that are already present", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "new.txt"), []byte("new\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		require.NoError(t, ApplyPatch(t.Context(), repo, patch))
		require.NoError(t, ApplyPatch(t.Context(), repo, patch))
		require.Equal(t, "two\n", read(t, filepath.Join(repo, "a.txt")))
	})

	t.Run("saves the patch and leaves the repository alone when it does not apply", func(t *testing.T) {
		repo, clone := setup(t)
		require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("mine\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		patch, err := Changes(t.Context(), clone, repo)
		require.NoError(t, err)

		err = ApplyPatch(t.Context(), repo, patch)
		require.Error(t, err)
		require.Equal(t, "mine\n", read(t, filepath.Join(repo, "a.txt")))

		_, saved, ok := strings.Cut(err.Error(), "saved to ")
		require.True(t, ok, err.Error())
		saved = strings.TrimSuffix(strings.Fields(saved)[0], ":")
		defer os.Remove(saved)
		require.Contains(t, read(t, saved), "+two")
	})
}
