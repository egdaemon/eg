package wasimod

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
	module := filepath.Join(t.TempDir(), "module.wasm")
	cmd := exec.Command("go", "build", "-o", module, "./.fixtures/module")
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("unable to build the wasip1 module: %v\n%s", err, b)
	}

	runtime := func(t *testing.T) wazero.Runtime {
		rt := wazero.NewRuntime(t.Context())
		t.Cleanup(func() { rt.Close(t.Context()) })
		wasi_snapshot_preview1.MustInstantiate(t.Context(), rt)
		return rt
	}

	load := func(t *testing.T, rt wazero.Runtime, dir string) (Modules, error) {
		return Load(t.Context(), rt, dir, wazero.NewModuleConfig().WithArgs("unknown"))
	}

	t.Run("a missing directory loads nothing", func(t *testing.T) {
		loaded, err := load(t, runtime(t), filepath.Join(t.TempDir(), "missing"))
		require.NoError(t, err)
		require.Empty(t, loaded)
	})

	t.Run("instantiates each module under its file name without running it", func(t *testing.T) {
		dir := t.TempDir()
		b, err := os.ReadFile(module)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cat.wasi"), b, 0600))
		rt := runtime(t)
		loaded, err := load(t, rt, dir)
		require.NoError(t, err)
		require.Len(t, loaded, 1)
		// the fixture exits on an unknown command, so a module that ran its start function would be closed.
		require.NotNil(t, rt.Module("cat"))
		require.False(t, rt.Module("cat").IsClosed())
		require.NotNil(t, rt.Module("cat").ExportedFunction("_start"))
	})

	t.Run("instantiates links under their own names", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Symlink(module, filepath.Join(dir, "cat.wasi")))
		require.NoError(t, os.Symlink(module, filepath.Join(dir, "tee.wasi")))
		rt := runtime(t)
		loaded, err := load(t, rt, dir)
		require.NoError(t, err)
		require.Len(t, loaded, 2)
		require.NotNil(t, rt.Module("cat"))
		require.NotNil(t, rt.Module("tee"))
	})

	t.Run("closes the modules it loaded", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Symlink(module, filepath.Join(dir, "cat.wasi")))
		rt := runtime(t)
		loaded, err := load(t, rt, dir)
		require.NoError(t, err)
		loaded.Close(t.Context())
		require.Nil(t, rt.Module("cat"))
	})

	t.Run("ignores files without the extension", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Symlink(module, filepath.Join(dir, "cat.wasm")))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "README"), []byte("notes"), 0600))
		loaded, err := load(t, runtime(t), dir)
		require.NoError(t, err)
		require.Empty(t, loaded)
	})

	t.Run("rejects a module that does not compile", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.wasi"), []byte("not wasm"), 0600))
		_, err := load(t, runtime(t), dir)
		require.ErrorContains(t, err, "unable to compile module")
	})

	t.Run("rejects a dangling link", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Symlink(filepath.Join(dir, "missing.wasm"), filepath.Join(dir, "cat.wasi")))
		_, err := load(t, runtime(t), dir)
		require.ErrorContains(t, err, "unable to resolve module")
	})

	t.Run("rejects two modules with the same name", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.Symlink(module, filepath.Join(dir, "cat.wasi")))
		b, err := os.ReadFile(module)
		require.NoError(t, err)
		rt := runtime(t)
		_, err = rt.InstantiateWithConfig(t.Context(), b, wazero.NewModuleConfig().WithName("cat").WithStartFunctions())
		require.NoError(t, err)
		_, err = load(t, rt, dir)
		require.ErrorContains(t, err, "unable to instantiate module")
	})
}
