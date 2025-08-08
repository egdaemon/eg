package wasix

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/tracex"
	"github.com/tetratelabs/wazero"
)

func Environ(mcfg wazero.ModuleConfig, environ ...string) wazero.ModuleConfig {
	for _, v := range environ {
		if k, v, ok := strings.Cut(v, "="); ok {
			mcfg = mcfg.WithEnv(k, v)
		}
	}

	return mcfg
}

func WarmCacheFromDirectoryTree(ctx context.Context, root string, cache wazero.CompilationCache) error {
	// Create a new WebAssembly Runtime.
	runtime := wazero.NewRuntimeWithConfig(
		ctx,
		wazero.NewRuntimeConfig().WithCompilationCache(cache),
	)

	return fs.WalkDir(os.DirFS(root), ".", func(path string, d fs.DirEntry, cause error) (err error) {
		var (
			wasi []byte
		)

		if cause != nil {
			return cause
		}

		if d.IsDir() { // recurse into directory
			return nil
		}

		tracex.Println("compiling module initiated", path)
		defer tracex.Println("compiling module completed", path)

		if wasi, err = os.ReadFile(filepath.Join(root, path)); err != nil {
			return errorsx.Wrap(err, "unable to open wasi")
		}

		c, err := runtime.CompileModule(ctx, wasi)
		if err != nil {
			return err
		}
		return c.Close(ctx)
	})

}

func WarmCacheDirectory(ctx context.Context, root, cache string) error {
	wazcache, err := wazero.NewCompilationCacheWithDir(cache)
	if err != nil {
		return errorsx.Wrap(err, "unable to prewarm cache")
	}

	return errorsx.Compact(
		WarmCacheFromDirectoryTree(ctx, root, wazcache),
		wazcache.Close(ctx),
	)
}

func WazCacheDir(roots ...string) string {
	return filepath.Join(filepath.Join(roots...), "wazcache")
}
