package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckpoint(t *testing.T) {
	t.Run("saves the working tree to the checkpoint ref and nothing else", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "new.txt"), []byte("new\n"), 0644))

		made, err := Checkpoint(t.Context(), clone)
		require.NoError(t, err)
		require.True(t, made)

		head, err := exec.Command("git", "-C", clone, "rev-parse", "HEAD").CombinedOutput()
		require.NoError(t, err, string(head))
		current, err := exec.Command("git", "-C", clone, "rev-parse", RefCurrent).CombinedOutput()
		require.NoError(t, err, string(current))
		require.Equal(t, strings.TrimSpace(string(head)), strings.TrimSpace(string(current)))

		a, err := exec.Command("git", "-C", clone, "show", RefCheckpoint+":a.txt").CombinedOutput()
		require.NoError(t, err, string(a))
		require.Equal(t, "two\n", string(a))
		n, err := exec.Command("git", "-C", clone, "show", RefCheckpoint+":new.txt").CombinedOutput()
		require.NoError(t, err, string(n))
		require.Equal(t, "new\n", string(n))

		// the clone's own state is as it was: the same head, the changes still uncommitted and unstaged.
		after, err := exec.Command("git", "-C", clone, "rev-parse", "HEAD").CombinedOutput()
		require.NoError(t, err, string(after))
		require.Equal(t, string(head), string(after))
		status, err := exec.Command("git", "-C", clone, "status", "--porcelain").CombinedOutput()
		require.NoError(t, err, string(status))
		require.Equal(t, " M a.txt\n?? new.txt\n", string(status))
	})

	t.Run("makes no checkpoint when nothing changed since the last one", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))

		// nothing differs from the head.
		made, err := Checkpoint(t.Context(), clone)
		require.NoError(t, err)
		require.False(t, made)
		require.Error(t, exec.Command("git", "-C", clone, "rev-parse", "--verify", "--quiet", RefCheckpoint).Run())

		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))
		made, err = Checkpoint(t.Context(), clone)
		require.NoError(t, err)
		require.True(t, made)

		first, err := exec.Command("git", "-C", clone, "rev-parse", RefCheckpoint).CombinedOutput()
		require.NoError(t, err, string(first))

		made, err = Checkpoint(t.Context(), clone)
		require.NoError(t, err)
		require.False(t, made)

		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("three\n"), 0644))
		made, err = Checkpoint(t.Context(), clone)
		require.NoError(t, err)
		require.True(t, made)

		second, err := exec.Command("git", "-C", clone, "rev-parse", RefCheckpoint).CombinedOutput()
		require.NoError(t, err, string(second))
		require.NotEqual(t, string(first), string(second))
	})
}
