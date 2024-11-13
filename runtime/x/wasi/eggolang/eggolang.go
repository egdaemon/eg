package eggolang

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/modfilex"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
)

func AutoCompile() eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) error {
		for gomod := range modfilex.FindModules(egenv.RootDirectory()) {
			if err := execx.MaybeRun(exec.CommandContext(ctx, "go", "build", fmt.Sprintf("%s/...", filepath.Dir(gomod)))); err != nil {
				return errorsx.Wrap(err, "unable to compile")
			}
		}
		return nil
	})
}

func AutoTest() eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) error {
		for gomod := range modfilex.FindModules(egenv.RootDirectory()) {
			if err := execx.MaybeRun(exec.CommandContext(ctx, "go", "test", fmt.Sprintf("%s/...", filepath.Dir(gomod)))); err != nil {
				return errorsx.Wrap(err, "unable to run tests")
			}
		}
		return nil
	})
}
