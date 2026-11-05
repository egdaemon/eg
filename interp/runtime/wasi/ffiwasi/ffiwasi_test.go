package ffiwasi

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

func TestLoad(t *testing.T) {
	// interptest loads module sets with this package, so the fixture is built here.
	module := filepath.Join(t.TempDir(), "module.wasm")
	cmd := exec.Command("go", "build", "-o", module, "./.fixtures/module")
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("unable to build the wasip1 module: %v\n%s", err, b)
	}

	load := func(t *testing.T, dir string) (*Set, error) {
		rt := wazero.NewRuntime(t.Context())
		t.Cleanup(func() { rt.Close(t.Context()) })
		wasi_snapshot_preview1.MustInstantiate(t.Context(), rt)
		return Load(t.Context(), rt, dir, wazero.NewFSConfig(), t.TempDir(), t.TempDir())
	}

	t.Run("a missing directory loads nothing", func(t *testing.T) {
		s, err := load(t, filepath.Join(t.TempDir(), "missing"))
		require.NoError(t, err)
		require.Empty(t, s.modules)
	})

	t.Run("names each module after its file", func(t *testing.T) {
		dir := t.TempDir()
		b, err := os.ReadFile(module)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cat.wasi"), b, 0600))
		s, err := load(t, dir)
		require.NoError(t, err)
		require.Contains(t, s.modules, "cat")
		require.Len(t, s.modules, 1)
	})

	t.Run("links share the module they point to", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Symlink(module, filepath.Join(dir, "cat.wasi")))
		require.NoError(t, os.Symlink(module, filepath.Join(dir, "tee.wasi")))
		s, err := load(t, dir)
		require.NoError(t, err)
		require.Len(t, s.modules, 2)
		require.Same(t, s.modules["cat"], s.modules["tee"])
	})

	t.Run("ignores files without the extension", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Symlink(module, filepath.Join(dir, "cat.wasm")))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README"), []byte("notes"), 0600))
		s, err := load(t, dir)
		require.NoError(t, err)
		require.Empty(t, s.modules)
	})

	t.Run("rejects a module that does not compile", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.wasi"), []byte("not wasm"), 0600))
		_, err := load(t, dir)
		require.ErrorContains(t, err, "unable to compile module")
	})

	t.Run("rejects a dangling link", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Symlink(filepath.Join(dir, "missing.wasm"), filepath.Join(dir, "cat.wasi")))
		_, err := load(t, dir)
		require.ErrorContains(t, err, "unable to resolve module")
	})
}
