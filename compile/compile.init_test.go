package compile_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg/compile"
	"github.com/stretchr/testify/require"
)

// stubgo places a fake go binary first on PATH that records each invocation
// and returns a function that reads the recorded argument lines.
func stubgo(t *testing.T) func() []string {
	t.Helper()
	dir := t.TempDir()
	invocations := filepath.Join(dir, "invocations")
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> \"%s\"\n", invocations)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go"), []byte(script), 0755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() []string {
		raw, err := os.ReadFile(invocations)
		require.NoError(t, err)
		return strings.Split(strings.TrimSpace(string(raw)), "\n")
	}
}

func TestInitGolang(t *testing.T) {
	t.Run("initializes_the_module_then_fetches_packages_with_a_writable_cache", func(t *testing.T) {
		invoked := stubgo(t)
		require.NoError(t, compile.InitGolang(t.Context(), t.TempDir(), "example.com/a", "example.com/b"))
		require.Equal(t, []string{
			"mod init eg/compute",
			"get -modcacherw -u example.com/a",
			"get -modcacherw -u example.com/b",
		}, invoked())
	})
}

func TestInitGolangModule(t *testing.T) {
	t.Run("aligns_the_module_with_the_enclosing_workspace_and_adds_it", func(t *testing.T) {
		invoked := stubgo(t)
		t.Setenv("GOWORK", "")
		root := t.TempDir()
		dir := filepath.Join(root, ".eg")
		require.NoError(t, os.MkdirAll(dir, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.22.1\n\nuse .\n"), 0600))

		require.NoError(t, compile.InitGolangModule(t.Context(), dir, "example.com/a"))
		require.Equal(t, []string{
			"mod init eg/compute",
			"mod edit -go=1.22.1",
			"work use ./.eg",
			"get -modcacherw -u example.com/a",
		}, invoked())
	})

	t.Run("prefers_the_workspace_named_by_gowork", func(t *testing.T) {
		invoked := stubgo(t)
		work := filepath.Join(t.TempDir(), "go.work")
		require.NoError(t, os.WriteFile(work, []byte("go 1.21.5\n"), 0600))
		t.Setenv("GOWORK", work)
		dir := filepath.Join(filepath.Dir(work), "mod")
		require.NoError(t, os.MkdirAll(dir, 0700))

		require.NoError(t, compile.InitGolangModule(t.Context(), dir, "example.com/a"))
		require.Equal(t, []string{
			"mod init eg/compute",
			"mod edit -go=1.21.5",
			"work use ./mod",
			"get -modcacherw -u example.com/a",
		}, invoked())
	})

	t.Run("creates_the_workspace_at_the_git_root_when_none_exists", func(t *testing.T) {
		invoked := stubgo(t)
		t.Setenv("GOWORK", "")
		root := t.TempDir()
		dir := filepath.Join(root, ".eg")
		require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0700))
		require.NoError(t, os.MkdirAll(dir, 0700))

		require.NoError(t, compile.InitGolangModule(t.Context(), dir, "example.com/a"))
		require.Equal(t, []string{
			"mod init eg/compute",
			"work init",
			"work use ./.eg",
			"get -modcacherw -u example.com/a",
		}, invoked())
	})

	t.Run("creates_the_workspace_in_the_current_directory_without_a_git_root", func(t *testing.T) {
		invoked := stubgo(t)
		t.Setenv("GOWORK", "")
		cwd := t.TempDir()
		dir := filepath.Join(cwd, "mod")
		require.NoError(t, os.MkdirAll(dir, 0700))
		t.Chdir(cwd)

		require.NoError(t, compile.InitGolangModule(t.Context(), dir, "example.com/a"))
		require.Equal(t, []string{
			"mod init eg/compute",
			"work init",
			"work use ./mod",
			"get -modcacherw -u example.com/a",
		}, invoked())
	})

	t.Run("behaves_like_initgolang_when_workspaces_are_off", func(t *testing.T) {
		invoked := stubgo(t)
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.22.1\n"), 0600))
		t.Setenv("GOWORK", "off")

		require.NoError(t, compile.InitGolangModule(t.Context(), root, "example.com/a"))
		require.Equal(t, []string{
			"mod init eg/compute",
			"get -modcacherw -u example.com/a",
		}, invoked())
	})
}

func TestInitPackages(t *testing.T) {
	t.Run("fetches_each_package_with_a_writable_cache", func(t *testing.T) {
		invoked := stubgo(t)
		require.NoError(t, compile.InitPackages(t.Context(), t.TempDir(), "-u", "example.com/a"))
		require.Equal(t, []string{"get -modcacherw -u example.com/a"}, invoked())
	})
}

func TestInitGolangTidy(t *testing.T) {
	t.Run("tidies_with_a_writable_cache", func(t *testing.T) {
		invoked := stubgo(t)
		require.NoError(t, compile.InitGolangTidy(t.Context(), t.TempDir()))
		require.Equal(t, []string{"mod tidy -modcacherw"}, invoked())
	})
}

func TestEnsureRequiredPackages(t *testing.T) {
	t.Run("fetches_defaults_and_extras_with_a_writable_cache", func(t *testing.T) {
		invoked := stubgo(t)
		require.NoError(t, compile.EnsureRequiredPackages(t.Context(), t.TempDir(), "example.com/extra"))
		require.Equal(t, []string{
			"get -modcacherw google.golang.org/genproto@latest github.com/egdaemon/eg/runtime/autowasinet github.com/egdaemon/eg/interp/events example.com/extra",
		}, invoked())
	})
}

func TestDropWorkspaceModule(t *testing.T) {
	t.Run("removes_the_module_from_the_enclosing_workspace", func(t *testing.T) {
		invoked := stubgo(t)
		t.Setenv("GOWORK", "")
		root := t.TempDir()
		dir := filepath.Join(root, ".eg")
		require.NoError(t, os.MkdirAll(dir, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(root, "go.work"), []byte("go 1.22.1\n\nuse ./.eg\n"), 0600))

		require.NoError(t, compile.DropWorkspaceModule(t.Context(), dir))
		require.Equal(t, []string{"work edit -dropuse ./.eg"}, invoked())
	})

	t.Run("does_nothing_without_a_workspace", func(t *testing.T) {
		stubgo(t)
		t.Setenv("GOWORK", "")
		dir := filepath.Join(t.TempDir(), ".eg")
		require.NoError(t, os.MkdirAll(dir, 0700))

		require.NoError(t, compile.DropWorkspaceModule(t.Context(), dir))
	})
}
