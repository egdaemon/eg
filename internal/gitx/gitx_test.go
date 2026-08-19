package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// initRepo creates a git repository at dir with a single commit containing
// the given committed files, then leaves an additional uncommitted/untracked
// file behind so tests can assert worktree checkouts only see committed
// state.
func initRepo(t *testing.T, dir string, committed map[string]string) {
	t.Helper()

	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %v: %s", args, out)
	}

	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")

	for name, content := range committed {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	run("add", ".")
	run("commit", "-q", "-m", "init")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("dirty"), 0644))
}
