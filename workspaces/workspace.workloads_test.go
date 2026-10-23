package workspaces_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func TestWorkloads(t *testing.T) {
	t.Run("finds every main-module package under dir", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.24\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644))

		var paths []string
		seq := workspaces.Workloads(t.Context(), dir)
		for d := range seq.Each(t.Context()) {
			paths = append(paths, d.Path)
		}
		require.NoError(t, seq.Err())

		slices.Sort(paths)
		require.Equal(t, []string{".", "sub"}, paths)
	})

	t.Run("empty dir yields nothing", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.24\n"), 0644))

		var paths []string
		seq := workspaces.Workloads(t.Context(), dir)
		for d := range seq.Each(t.Context()) {
			paths = append(paths, d.Path)
		}
		require.NoError(t, seq.Err())
		require.Empty(t, paths)
	})
}

func TestBuiltinWorkloads(t *testing.T) {
	archive := fstest.MapFS{
		".builtin/main.go":            {Data: []byte("package main\n")},
		".builtin/a/main.go":          {Data: []byte("package main\n")},
		".builtin/a/main_test.go":     {Data: []byte("package main\n")},
		".builtin/b/c/main.go":        {Data: []byte("package main\n")},
		".builtin/b/c/extra.go":       {Data: []byte("package main\n")},
		".builtin/onlytest/x_test.go": {Data: []byte("package main\n")},
		".builtin/empty/.keep":        {Data: []byte("")},
	}

	found, err := workspaces.BuiltinWorkloads(t.Context(), archive)
	require.NoError(t, err)

	paths := make([]string, 0, len(found))
	for _, d := range found {
		paths = append(paths, d.Path)
	}
	slices.Sort(paths)
	require.Equal(t, []string{".builtin/a", ".builtin/b/c"}, paths)
}
