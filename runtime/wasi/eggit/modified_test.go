package eggit

import (
	"fmt"
	"os/exec"
	"sync"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

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

func newTestModified(t *testing.T, dir string, getenv func(string) string) *modified {
	t.Helper()
	m := &modified{o: sync.Once{}, runtime: shell.NewLocal(), getenv: getenv}
	testx.Tempenvvar(_eg.EnvComputeWorkingDirectory, dir, func() {
		testx.Tempenvvar(_eg.EnvComputeRuntimeDirectory, dir, func() {
			require.NoError(t, m.detect(t.Context()))
		})
	})
	return m
}

func TestModifiedDetect(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		dir := t.TempDir()
		gitInit(t, dir)
		baseCommit := gitRevParse(t, dir, "HEAD")

		cmd := exec.Command("mkdir", "-p", "src")
		cmd.Dir = dir
		require.NoError(t, cmd.Run())
		gitCommitFiles(t, dir, []string{"src/main.go"}, "add main.go")
		headCommit := gitRevParse(t, dir, "HEAD")

		getenv := envx.NewEnvironFromStrings(
			fmt.Sprintf("%s=%s", _eg.EnvGitHeadCommit, headCommit),
			fmt.Sprintf("%s=%s", _eg.EnvGitBaseCommit, baseCommit),
		)

		m := newTestModified(t, dir, getenv.Map)
		require.Contains(t, m.changed, "src/main.go")
	})

	t.Run("multiple files", func(t *testing.T) {
		dir := t.TempDir()
		gitInit(t, dir)
		baseCommit := gitRevParse(t, dir, "HEAD")

		for _, d := range []string{"src", "docs"} {
			cmd := exec.Command("mkdir", "-p", d)
			cmd.Dir = dir
			require.NoError(t, cmd.Run())
		}
		gitCommitFiles(t, dir, []string{"src/main.go", "src/util.go", "docs/readme.md"}, "add files")
		headCommit := gitRevParse(t, dir, "HEAD")

		getenv := envx.NewEnvironFromStrings(
			fmt.Sprintf("%s=%s", _eg.EnvGitHeadCommit, headCommit),
			fmt.Sprintf("%s=%s", _eg.EnvGitBaseCommit, baseCommit),
		)

		m := newTestModified(t, dir, getenv.Map)
		require.Contains(t, m.changed, "src/main.go")
		require.Contains(t, m.changed, "src/util.go")
		require.Contains(t, m.changed, "docs/readme.md")
	})

	t.Run("same commit", func(t *testing.T) {
		dir := t.TempDir()
		gitInit(t, dir)
		commit := gitRevParse(t, dir, "HEAD")

		getenv := envx.NewEnvironFromStrings(
			fmt.Sprintf("%s=%s", _eg.EnvGitHeadCommit, commit),
			fmt.Sprintf("%s=%s", _eg.EnvGitBaseCommit, commit),
		)

		m := newTestModified(t, dir, getenv.Map)
		require.Empty(t, m.changed)
	})

	t.Run("empty head commit", func(t *testing.T) {
		dir := t.TempDir()
		gitInit(t, dir)

		getenv := envx.NewEnvironFromStrings()

		m := newTestModified(t, dir, getenv.Map)
		require.Empty(t, m.changed)
	})
}

func TestModifiedChanged(t *testing.T) {
	t.Run("matches prefix", func(t *testing.T) {
		dir := t.TempDir()
		gitInit(t, dir)
		baseCommit := gitRevParse(t, dir, "HEAD")

		for _, d := range []string{"src", "docs"} {
			cmd := exec.Command("mkdir", "-p", d)
			cmd.Dir = dir
			require.NoError(t, cmd.Run())
		}
		gitCommitFiles(t, dir, []string{"src/main.go", "docs/readme.md"}, "add files")
		headCommit := gitRevParse(t, dir, "HEAD")

		getenv := envx.NewEnvironFromStrings(
			fmt.Sprintf("%s=%s", _eg.EnvGitHeadCommit, headCommit),
			fmt.Sprintf("%s=%s", _eg.EnvGitBaseCommit, baseCommit),
		)

		m := newTestModified(t, dir, getenv.Map)

		testx.Tempenvvar(_eg.EnvComputeWorkingDirectory, dir, func() {
			testx.Tempenvvar(_eg.EnvComputeRuntimeDirectory, dir, func() {
				require.True(t, m.Changed("src/")(t.Context()))
				require.True(t, m.Changed("docs/")(t.Context()))
				require.False(t, m.Changed("pkg/")(t.Context()))
			})
		})
	})

	t.Run("no paths returns true", func(t *testing.T) {
		dir := t.TempDir()
		gitInit(t, dir)
		baseCommit := gitRevParse(t, dir, "HEAD")

		gitCommitFiles(t, dir, []string{"file.txt"}, "add file")
		headCommit := gitRevParse(t, dir, "HEAD")

		getenv := envx.NewEnvironFromStrings(
			fmt.Sprintf("%s=%s", _eg.EnvGitHeadCommit, headCommit),
			fmt.Sprintf("%s=%s", _eg.EnvGitBaseCommit, baseCommit),
		)

		m := newTestModified(t, dir, getenv.Map)

		testx.Tempenvvar(_eg.EnvComputeWorkingDirectory, dir, func() {
			testx.Tempenvvar(_eg.EnvComputeRuntimeDirectory, dir, func() {
				require.True(t, m.Changed()(t.Context()))
			})
		})
	})
}
