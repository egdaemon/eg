package workspaces_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

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
