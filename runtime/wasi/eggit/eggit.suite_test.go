package eggit

import (
	"os/exec"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/stretchr/testify/require"
)

func testcommit1() commit {
	return commit{
		Hash: nhash("211a6f2a398230a29830a31a9cbf422be74d407f"),
		Committer: signature{
			Name:  "EGd engineering",
			Email: "engineering@egdaemon.com",
			When:  errorsx.Must(time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")),
		},
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run(), args)
	}
}

func gitRevParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	return string(out[:len(out)-1])
}

func gitCommitFiles(t *testing.T, dir string, files []string, msg string) {
	t.Helper()
	for _, f := range files {
		cmd := exec.Command("touch", f)
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
	}
	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", msg},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		require.NoError(t, cmd.Run(), args)
	}
}
