package workspaces_test

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestNewLocalRoot(t *testing.T) {
	t.Run("roots in the workload directory of the repository when not launched by another workload", func(t *testing.T) {
		cwd := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(cwd, ".eg"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(cwd, ".eg", "main.go"), []byte("package main"), 0644))
		t.Setenv(eg.EnvComputeWorkloadDirectory, "")

		ws, err := workspaces.NewLocal(t.Context(), uuid.Max, sha256.New(), cwd, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(cwd, eg.WorkloadDirectory, egunsafe.ModuleRuntimeDirectory(uuid.Max)), ws.Root)
	})

	t.Run("roots beside the root of the workload that launched it", func(t *testing.T) {
		shared := filepath.Join(t.TempDir(), eg.WorkloadDirectory)
		cwd := filepath.Join(shared, "m.aaaaaaaa.d", eg.WorkingDirectory)
		require.NoError(t, os.MkdirAll(filepath.Join(cwd, ".eg"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(cwd, ".eg", "main.go"), []byte("package main"), 0644))
		t.Setenv(eg.EnvComputeWorkloadDirectory, filepath.Join(shared, "m.aaaaaaaa.d"))

		ws, err := workspaces.NewLocal(t.Context(), uuid.Max, sha256.New(), cwd, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(shared, egunsafe.ModuleRuntimeDirectory(uuid.Max)), ws.Root)
		require.Equal(t, filepath.Join(ws.Root, eg.RuntimeDirectory), ws.RuntimeDir)
	})

	t.Run("nested workloads share the directory of their roots at any depth", func(t *testing.T) {
		shared := filepath.Join(t.TempDir(), eg.WorkloadDirectory)
		cwd := filepath.Join(shared, "m.aaaaaaaa.d", eg.WorkingDirectory)
		require.NoError(t, os.MkdirAll(filepath.Join(cwd, ".eg"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(cwd, ".eg", "main.go"), []byte("package main"), 0644))
		t.Setenv(eg.EnvComputeWorkloadDirectory, filepath.Join(shared, "m.aaaaaaaa.d"))

		first, err := workspaces.NewLocal(t.Context(), uuid.Must(uuid.FromString("01890a5d-ac96-774b-bcce-b302099a1111")), sha256.New(), cwd, "")
		require.NoError(t, err)

		// the first is the parent of the second, as baremetal provides its root to the workloads it runs.
		t.Setenv(eg.EnvComputeWorkloadDirectory, first.Root)

		second, err := workspaces.NewLocal(t.Context(), uuid.Must(uuid.FromString("01890a5d-ac96-774b-bcce-b302099a2222")), sha256.New(), first.WorkingDir, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Dir(first.Root), filepath.Dir(second.Root))
		require.Equal(t, len(first.RuntimeDir), len(second.RuntimeDir))
	})

	t.Run("roots in the workload directory of the repository when the launching workload is not in a workload directory", func(t *testing.T) {
		cwd := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(cwd, ".eg"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(cwd, ".eg", "main.go"), []byte("package main"), 0644))
		// containers provide the default root directory, whose parent is the filesystem root.
		t.Setenv(eg.EnvComputeWorkloadDirectory, eg.DefaultRootDirectory())

		ws, err := workspaces.NewLocal(t.Context(), uuid.Max, sha256.New(), cwd, "")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(cwd, eg.WorkloadDirectory, egunsafe.ModuleRuntimeDirectory(uuid.Max)), ws.Root)
	})

	t.Run("the repository is the working directory, not the root of the launching workload", func(t *testing.T) {
		shared := filepath.Join(t.TempDir(), eg.WorkloadDirectory)
		cwd := filepath.Join(shared, "m.aaaaaaaa.d", eg.WorkingDirectory)
		require.NoError(t, os.MkdirAll(filepath.Join(cwd, ".eg"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(cwd, ".eg", "main.go"), []byte("package main"), 0644))
		t.Setenv(eg.EnvComputeWorkloadDirectory, filepath.Join(shared, "m.aaaaaaaa.d"))

		ws, err := workspaces.NewLocal(t.Context(), uuid.Max, sha256.New(), cwd, "")
		require.NoError(t, err)

		target, err := os.Readlink(ws.WorkingDir)
		require.NoError(t, err)
		require.Equal(t, cwd, target)
	})
}
