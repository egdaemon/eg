package egcargo

import (
	"context"
	"fmt"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strings"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func CacheDirectory(dirs ...string) string {
	return egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "cargo", filepath.Join(dirs...))
}

// attempt to build the rust environment that sets up
// the cargo environment for caching.
func env() ([]string, error) {
	return envx.Build().FromEnv(os.Environ()...).
		Var("CARGO_HOME", CacheDirectory()).
		Environ()
}

// attempt to build the rust environment that sets up
// the cargo environment for caching.
func Env() []string {
	return errorsx.Must(env())
}

// Create a shell runtime that properly
// sets up the cargo environment for caching.
func Runtime() shell.Command {
	return shell.Runtime().
		EnvironFrom(
			errorsx.Must(env())...,
		)
}

func AutoTest() eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			cenv []string
		)

		if cenv, err = env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(cenv...)

		for croot := range findroot(egenv.WorkingDirectory()) {
			cmd := stringsx.Join(" ", "cargo", "test")
			if err := shell.Run(ctx, runtime.New(cmd).Directory(croot)); err != nil {
				return errorsx.Wrap(err, "unable to run tests")
			}
		}

		return nil
	})
}

func findroot(root string) iter.Seq[string] {
	tree := os.DirFS(root)

	return func(yield func(string) bool) {
		err := fs.WalkDir(tree, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// ignore hidden directories. short term hack.
			if strings.HasPrefix(filepath.Base(path), ".") && path != "." && d.IsDir() {
				return fs.SkipDir
			}

			// recurse into directories.
			if d.IsDir() {
				return nil
			}

			if filepath.Base(path) != "Cargo.toml" {
				return nil
			}

			if !yield(filepath.Join(root, path)) {
				return fmt.Errorf("failed to yield path: %s", filepath.Join(root, path))
			}

			return nil
		})

		errorsx.Log(errorsx.Wrap(err, "unable to yield path"))
	}
}
