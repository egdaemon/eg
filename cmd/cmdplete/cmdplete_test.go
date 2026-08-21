package cmdplete_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egdaemon/eg/cmd/cmdplete"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestWorkloadPredict(t *testing.T) {
	t.Run("returns every main-module package under root", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.24\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644))
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644))

		results := cmdplete.NewWorkload(dir).Predict(complete.Args{})
		slices.Sort(results)
		require.Equal(t, []string{".", "sub"}, results)
	})

	t.Run("nonexistent root returns nil instead of panicking", func(t *testing.T) {
		results := cmdplete.NewWorkload(filepath.Join(t.TempDir(), "does-not-exist")).Predict(complete.Args{})
		require.Nil(t, results)
	})
}
