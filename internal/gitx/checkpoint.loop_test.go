package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckpointLoop(t *testing.T) {
	t.Run("checkpoints on the interval and a zero interval starts none", func(t *testing.T) {
		repo := t.TempDir()
		initRepo(t, repo, map[string]string{"a.txt": "one\n"})
		clone := filepath.Join(t.TempDir(), "clone")
		require.NoError(t, LocalClone(t.Context(), repo, clone, "main", "HEAD"))
		require.NoError(t, os.WriteFile(filepath.Join(clone, "a.txt"), []byte("two\n"), 0644))

		CheckpointLoop(t.Context(), clone, 0)()
		time.Sleep(50 * time.Millisecond)
		require.Error(t, exec.Command("git", "-C", clone, "rev-parse", "--verify", "--quiet", RefCheckpoint).Run())

		stop := CheckpointLoop(t.Context(), clone, 20*time.Millisecond)
		require.Eventually(t, func() bool {
			return exec.Command("git", "-C", clone, "rev-parse", "--verify", "--quiet", RefCheckpoint).Run() == nil
		}, 10*time.Second, 20*time.Millisecond)
		stop()
	})
}
