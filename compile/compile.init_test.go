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
