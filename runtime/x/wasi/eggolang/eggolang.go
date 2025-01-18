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

var BuildOption = boption(nil)

type buildOption struct {
	flags   []string
	environ []string
	bctx    build.Context
}

type boption func(*buildOption)

func (boption) Debug(b bool) boption {
	return func(o *buildOption) {
		if !b {
			return
		}
		o.flags = append(o.flags, "-x")
	}
}

func (boption) Tags(tags ...string) boption {
	return func(o *buildOption) {
		o.bctx.BuildTags = tags
	}
}

func Build(opts ...boption) (b buildOption) {
	return langx.Clone(b, opts...)
}

// escape hatch for setting command line flags.
// useful for flags not explicitly implemented by this package.
func (boption) Flags(flags ...string) boption {
	return func(o *buildOption) {
		o.flags = append(o.flags, flags...)
	}
}

// escape hatch for setting command environment variables.
// useful for flags not explicitly implemented by this package.
func (boption) Environ(envvars ...string) boption {
	return func(o *buildOption) {
		o.environ = append(o.environ, envvars...)
	}
}

func (t buildOption) options() (opts []string) {
	copy(opts, t.flags)
	if len(t.bctx.BuildTags) > 0 {
		opts = append(opts, fmt.Sprintf("-tags=%s", strings.Join(t.bctx.BuildTags, ",")))
	}

	return opts
}

var InstallOption = ioption(nil)

type ioption func(*installOption)

type installOption struct {
	buildOption
}

func (ioption) BuildOptions(b buildOption) ioption {
	return func(o *installOption) {
		o.buildOption = b
	}
}

func AutoInstall(options ...toption) eg.OpFn {
	var (
		opts testOption
	)

	opts = langx.Clone(opts, options...)
	flags := stringsx.Join(" ", opts.buildOption.options()...)

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			goenv []string
		)

		if goenv, err = Env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(goenv...)

		for gomod := range modfilex.FindModules(egenv.WorkingDirectory()) {
			cmd := stringsx.Join(" ", "go", "install", flags, fmt.Sprintf("%s/...", filepath.Dir(gomod)))
			if err := shell.Run(ctx, runtime.New(cmd)); err != nil {
				return errorsx.Wrap(err, "unable to run tests")
			}
		}

		return nil
	})
}

var CompileOption = coption(nil)

type coption func(*compileOption)

func (coption) BuildOptions(b buildOption) coption {
	return func(o *compileOption) {
		o.buildOption = b
	}
}

type compileOption struct {
	buildOption
}

func AutoCompile(options ...coption) eg.OpFn {
	var (
		opts compileOption
	)

	opts = langx.Clone(opts, options...)
	flags := stringsx.Join(" ", opts.buildOption.options()...)

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			goenv []string
		)

		if goenv, err = Env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(goenv...).EnvironFrom(opts.buildOption.environ...)

		for gomod := range modfilex.FindModules(egenv.WorkingDirectory()) {
			cmd := stringsx.Join(" ", "go", "build", flags, fmt.Sprintf("%s/...", filepath.Dir(gomod)))
			if err := shell.Run(ctx, runtime.New(cmd)); err != nil {
				return errorsx.Wrap(err, "unable to compile")
			}
		}

		return nil
	})
}

var TestOption = toption(nil)

type toption func(*testOption)

func (toption) BuildOptions(b buildOption) toption {
	return func(o *testOption) {
		o.buildOption = b
	}
}

// Randomize execution order of the tests using the provided seed value.
// defaults to on.
// 0 = off
// 1 = on
// N = seed
func (toption) Randomize(seed int) toption {
	return func(o *testOption) {
		switch seed {
		case 0:
			o.randomize = "-shuffle off"
		case 1:
			o.randomize = "-shuffle on"
		default:
			o.randomize = fmt.Sprintf("-shuffle %d", seed)
		}
	}
}

// The number of times to run the test suite.
// 0 uses go test default.
func (toption) Count(n int) toption {
	return func(o *testOption) {
		switch n {
		case 0:
			o.count = ""
		default:
			o.count = fmt.Sprintf("-count %d", n)
		}
	}
}

// used to disable golang test caching.
// short for Count(1) see go help test.
// will only be applied if count is unset.
func (t toption) NoCache(o *testOption) {
	if stringsx.Present(o.count) {
		return
	}

	t.Count(1)(o)
}

func (toption) Verbose(b bool) toption {
	return func(o *testOption) {
		if b {
			o.verbose = "-v"
		} else {
			o.verbose = ""
		}
	}
}

type testOption struct {
	buildOption
	count     string
	randomize string
	verbose   string
}

func (t testOption) options() (dst []string) {
	ignoreEmpty := func(dst []string, o string) []string {
		if stringsx.Blank(o) {
			return dst
		}

		return append(dst, o)
	}
	dst = t.buildOption.options()
	dst = ignoreEmpty(dst, t.randomize)
	dst = ignoreEmpty(dst, t.count)
	dst = ignoreEmpty(dst, t.verbose)

	return dst
}

func AutoTest(options ...toption) eg.OpFn {
	var (
		opts = testOption{}
	)

	opts = langx.Clone(opts, options...)
	flags := stringsx.Join(" ", opts.options()...)

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			goenv []string
		)

		if goenv, err = Env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(goenv...)

		for gomod := range modfilex.FindModules(egenv.WorkingDirectory()) {
			cmd := stringsx.Join(" ", "go", "test", flags, fmt.Sprintf("%s/...", filepath.Dir(gomod)))
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
