package eggolang

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/modfilex"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffiexec"
)

func AutoCompile() eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) error {
		// golang's wasm implementation doesn't have a reasonable default in place. it defaults to returning not found.
		for gomod := range modfilex.FindModules(egenv.RootDirectory()) {
			err := ffiexec.Command(ctx, egenv.RootDirectory(), os.Environ(), "go", []string{
				"build",
				fmt.Sprintf("%s/...", filepath.Dir(gomod)),
			})
			if err != nil {
				return errorsx.Wrap(err, "unable to compile")
			}
		}

		return nil
	})
}

func AutoTest() eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) error {
		// golang's wasm implementation doesn't have a reasonable default in place. it defaults to returning not found.
		for gomod := range modfilex.FindModules(egenv.RootDirectory()) {
			err := ffiexec.Command(ctx, egenv.RootDirectory(), os.Environ(), "go", []string{
				"test",
				fmt.Sprintf("%s/...", filepath.Dir(gomod)),
			})
			if err != nil {
				return errorsx.Wrap(err, "unable to run tests")
			}
		}
		return nil
	})
}
