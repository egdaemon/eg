package eggit

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/stretchr/testify/require"
)

func TestModified(t *testing.T) {
	t.Run("options", func(t *testing.T) {
		t.Run("defaults", func(t *testing.T) {
			m := NewModified()
			require.NotNil(t, m.runtime)
			require.NotNil(t, m.getenv)
			require.NotNil(t, m.workdir)
			require.NotNil(t, m.rundir)
		})

		t.Run("overrides", func(t *testing.T) {
			m := NewModified(
				ModifiedOptionGetenv(func(k string) string { return "v:" + k }),
				ModifiedOptionWorkingDirectory("/w"),
				ModifiedOptionRuntimeDirectory("/r"),
			)
			require.Equal(t, "v:x", m.getenv("x"))
			require.Equal(t, "/w/a", m.workdir("a"))
			require.Equal(t, "/r/eg.git.mod", m.rundir("eg.git.mod"))
		})
	})

	t.Run("detect", func(t *testing.T) {
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

			m := NewModified(
				ModifiedOptionRuntime(shell.NewLocal()),
				ModifiedOptionGetenv(getenv.Map),
				ModifiedOptionWorkingDirectory(dir),
				ModifiedOptionRuntimeDirectory(dir),
			)
			require.NoError(t, m.detect(t.Context()))
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

			m := NewModified(
				ModifiedOptionRuntime(shell.NewLocal()),
				ModifiedOptionGetenv(getenv.Map),
				ModifiedOptionWorkingDirectory(dir),
				ModifiedOptionRuntimeDirectory(dir),
			)
			require.NoError(t, m.detect(t.Context()))
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

			m := NewModified(
				ModifiedOptionRuntime(shell.NewLocal()),
				ModifiedOptionGetenv(getenv.Map),
				ModifiedOptionWorkingDirectory(dir),
				ModifiedOptionRuntimeDirectory(dir),
			)
			require.NoError(t, m.detect(t.Context()))
			require.Empty(t, m.changed)
		})

		t.Run("unknown commit treated as unchanged", func(t *testing.T) {
			dir := t.TempDir()
			gitInit(t, dir)
			baseCommit := gitRevParse(t, dir, "HEAD")

			getenv := envx.NewEnvironFromStrings(
				fmt.Sprintf("%s=%s", _eg.EnvGitHeadCommit, "211a6f2a398230a29830a31a9cbf422be74d407f"),
				fmt.Sprintf("%s=%s", _eg.EnvGitBaseCommit, baseCommit),
			)

			m := NewModified(
				ModifiedOptionRuntime(shell.NewLocal()),
				ModifiedOptionGetenv(getenv.Map),
				ModifiedOptionWorkingDirectory(dir),
				ModifiedOptionRuntimeDirectory(dir),
			)
			require.NoError(t, m.detect(t.Context()))
			require.Empty(t, m.changed)
		})

		t.Run("commits without file changes", func(t *testing.T) {
			dir := t.TempDir()
			gitInit(t, dir)
			baseCommit := gitRevParse(t, dir, "HEAD")

			cmd := exec.Command("git", "commit", "--allow-empty", "-m", "empty")
			cmd.Dir = dir
			require.NoError(t, cmd.Run())
			headCommit := gitRevParse(t, dir, "HEAD")

			getenv := envx.NewEnvironFromStrings(
				fmt.Sprintf("%s=%s", _eg.EnvGitHeadCommit, headCommit),
				fmt.Sprintf("%s=%s", _eg.EnvGitBaseCommit, baseCommit),
			)

			m := NewModified(
				ModifiedOptionRuntime(shell.NewLocal()),
				ModifiedOptionGetenv(getenv.Map),
				ModifiedOptionWorkingDirectory(dir),
				ModifiedOptionRuntimeDirectory(dir),
			)
			require.NoError(t, m.detect(t.Context()))
			require.Empty(t, m.changed)
		})

		t.Run("unwritable runtime directory", func(t *testing.T) {
			dir := t.TempDir()
			gitInit(t, dir)
			baseCommit := gitRevParse(t, dir, "HEAD")
			gitCommitFiles(t, dir, []string{"file.txt"}, "add file")
			headCommit := gitRevParse(t, dir, "HEAD")

			getenv := envx.NewEnvironFromStrings(
				fmt.Sprintf("%s=%s", _eg.EnvGitHeadCommit, headCommit),
				fmt.Sprintf("%s=%s", _eg.EnvGitBaseCommit, baseCommit),
			)

			m := NewModified(
				ModifiedOptionRuntime(shell.NewLocal()),
				ModifiedOptionGetenv(getenv.Map),
				ModifiedOptionWorkingDirectory(dir),
				ModifiedOptionRuntimeDirectory(filepath.Join(dir, "missing")),
			)
			require.ErrorContains(t, m.detect(t.Context()), "git diff failed")
		})

		t.Run("empty head commit", func(t *testing.T) {
			dir := t.TempDir()
			gitInit(t, dir)

			getenv := envx.NewEnvironFromStrings()

			m := NewModified(
				ModifiedOptionRuntime(shell.NewLocal()),
				ModifiedOptionGetenv(getenv.Map),
				ModifiedOptionWorkingDirectory(dir),
				ModifiedOptionRuntimeDirectory(dir),
			)
			require.NoError(t, m.detect(t.Context()))
			require.Empty(t, m.changed)
		})
	})

	t.Run("changed", func(t *testing.T) {
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

			m := NewModified(
				ModifiedOptionRuntime(shell.NewLocal()),
				ModifiedOptionGetenv(getenv.Map),
				ModifiedOptionWorkingDirectory(dir),
				ModifiedOptionRuntimeDirectory(dir),
			)

			require.True(t, m.Changed("src/")(t.Context()))
			require.True(t, m.Changed("docs/")(t.Context()))
			require.False(t, m.Changed("pkg/")(t.Context()))
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

			m := NewModified(
				ModifiedOptionRuntime(shell.NewLocal()),
				ModifiedOptionGetenv(getenv.Map),
				ModifiedOptionWorkingDirectory(dir),
				ModifiedOptionRuntimeDirectory(dir),
			)

			require.True(t, m.Changed()(t.Context()))
		})
	})
}
