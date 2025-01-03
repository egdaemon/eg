package eggolang

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/modfilex"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

type CompileOption func(*compileOption)

func CompileOptionTags(tags ...string) CompileOption {
	return func(c *compileOption) {
		c.bctx.BuildTags = tags
	}
}

type compileOption struct {
	bctx build.Context
}

func AutoCompile(options ...CompileOption) eg.OpFn {
	var (
		tags  string
		copts compileOption
	)
	copts = langx.Clone(copts, options...)
	if len(copts.bctx.BuildTags) > 0 {
		tags = fmt.Sprintf("-tags=%s", strings.Join(copts.bctx.BuildTags, ","))
	}

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			goenv []string
		)

		if goenv, err = Env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(goenv...)

		// golang's wasm implementation doesn't have a reasonable default in place. it defaults to returning not found.
		for gomod := range modfilex.FindModules(egenv.RootDirectory()) {
			cmd := stringsx.Join(" ", "go", "build", tags, fmt.Sprintf("%s/...", filepath.Dir(gomod)))
			if err := shell.Run(ctx, runtime.New(cmd)); err != nil {
				return errorsx.Wrap(err, "unable to compile")
			}
		}

		return nil
	})
}

type TestOption func(*testOption)

func TestOptionTags(tags ...string) TestOption {
	return func(c *testOption) {
		c.bctx.BuildTags = tags
	}
}

type testOption struct {
	bctx build.Context
}

func AutoTest(options ...TestOption) eg.OpFn {
	var (
		tags string
		opts testOption
	)

	opts = langx.Clone(opts, options...)
	if len(opts.bctx.BuildTags) > 0 {
		tags = fmt.Sprintf("-tags=%s", strings.Join(opts.bctx.BuildTags, ","))
	}

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			goenv []string
		)

		if goenv, err = Env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(goenv...)

		// golang's wasm implementation doesn't have a reasonable default in place. it defaults to returning not found.
		for gomod := range modfilex.FindModules(egenv.RootDirectory()) {
			cmd := stringsx.Join(" ", "go", "test", tags, fmt.Sprintf("%s/...", filepath.Dir(gomod)))
			if err := shell.Run(ctx, runtime.New(cmd)); err != nil {
				return errorsx.Wrap(err, "unable to run tests")
			}
		}

		return nil
	})
}

func CacheDirectory(dirs ...string) string {
	return egenv.CacheDirectory(".eg", "golang", filepath.Join(dirs...))
}

func CacheBuildDirectory() string {
	return CacheDirectory("build")
}

func CacheModuleDirectory() string {
	return CacheDirectory("mod")
}

// attempt to build the golang environment that sets up
// the golang environment for caching.
func Env() ([]string, error) {
	return envx.Build().FromEnv(os.Environ()...).
		Var("GOCACHE", CacheBuildDirectory()).
		Var("GOMODCACHE", CacheModuleDirectory()).
		Environ()
}

// Create a shell runtime that properly
// sets up the golang environment for caching.
func Runtime() shell.Command {
	return shell.Runtime().
		EnvironFrom(
			errorsx.Must(Env())...,
		)
}
