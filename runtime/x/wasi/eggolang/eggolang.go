package eggolang

import (
	"context"
	"fmt"
	"go/build"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/contextx"
	"github.com/egdaemon/eg/internal/coverage/golangcov"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/modfilex"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/fficoverage"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

var BuildOption = boption(nil)

type buildOption struct {
	flags   []string
	environ []string
	bctx    build.Context
	timeout time.Duration
}

type boption func(*buildOption)

// provide a timeout for the command.
func (boption) Timeout(d time.Duration) boption {
	return func(o *buildOption) {
		o.timeout = d
	}
}

// enable debug flag
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

func (boption) WorkingDirectory(s string) boption {
	return func(o *buildOption) {
		o.bctx.Dir = s
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

func AutoInstall(options ...ioption) eg.OpFn {
	var (
		opts installOption
	)

	opts = langx.Clone(opts, options...)
	flags := stringsx.Join(" ", opts.buildOption.options()...)

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			goenv []string
		)

		if goenv, err = env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(goenv...)
		timeout := timex.DurationMin(contextx.Until(ctx), timex.DurationFirstNonZero(opts.timeout, shell.DefaultTimeout))

		for gomod := range modfilex.FindModules(stringsx.DefaultIfBlank(opts.bctx.Dir, egenv.WorkingDirectory())) {
			cmd := stringsx.Join(" ", slicesx.Filter(func(s string) bool { return stringsx.Present(s) }, "go", "-C", filepath.Dir(gomod), "install", flags, "./...")...)
			log.Println("DERP DERP", cmd)
			if err := shell.Run(ctx, runtime.New(cmd).Debug().Timeout(timeout)); err != nil {
				return errorsx.Wrap(err, "unable to install")
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

		if goenv, err = env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(goenv...).EnvironFrom(opts.buildOption.environ...)
		timeout := timex.DurationMin(contextx.Until(ctx), timex.DurationFirstNonZero(opts.timeout, shell.DefaultTimeout))

		for gomod := range modfilex.FindModules(stringsx.DefaultIfBlank(opts.bctx.Dir, egenv.WorkingDirectory())) {
			cmd := stringsx.Join(" ", "go", "-C", filepath.Dir(gomod), "build", flags, "./...")
			if err := shell.Run(ctx, runtime.New(cmd).Timeout(timeout)); err != nil {
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

// set environment variables for this test specifically.
func (toption) Environ(envvars ...string) toption {
	return func(o *testOption) {
		o.environ = append(o.environ, envvars...)
	}
}

type testOption struct {
	buildOption
	coverage  string // coverage mode
	count     string
	randomize string
	verbose   string
	environ   []string
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
	dst = append(dst, t.coverage)

	return dst
}

func AutoTest(options ...toption) eg.OpFn {
	var (
		opts = testOption{
			coverage: "-covermode count",
		}
	)

	opts = langx.Clone(opts, options...)
	flags := stringsx.Join(" ", opts.options()...)

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			goenv []string
		)

		covpath := coveragedir()
		if err := shell.Run(ctx, shell.Newf("mkdir -p %s", covpath)); err != nil {
			return errorsx.Wrap(err, "unable to run tests")
		}

		if goenv, err = env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(goenv...).EnvironFrom(opts.environ...)
		timeout := timex.DurationMin(contextx.Until(ctx), timex.DurationFirstNonZero(opts.timeout, shell.DefaultTimeout))

		for gomod := range modfilex.FindModules(egenv.WorkingDirectory()) {
			cmd := stringsx.Join(" ", "go", "-C", filepath.Dir(gomod), "test", fmt.Sprintf("-timeout=%s", timeout), flags, fmt.Sprintf("-coverprofile %s", filepath.Join(covpath, md5x.String(gomod))), "./...")
			if err := shell.Run(ctx, runtime.New(cmd).Timeout(timeout+time.Second)); err != nil {
				return errorsx.Wrap(err, "unable to run tests")
			}
		}

		return nil
	})
}

// Record the coverage profile into the duckdb database.
func RecordCoverage(ctx context.Context, _ eg.Op) (err error) {
	covpath := coveragedir()

	// recover metrics
	batch := make([]*events.Coverage, 0, 128)
	for rep, err := range golangcov.Coverage(ctx, covpath) {
		if err != nil {
			return err
		}

		batch = append(batch, rep)

		if len(batch) == cap(batch) {
			if err := fficoverage.Report(ctx, batch...); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}

	if err := fficoverage.Report(ctx, batch...); err != nil {
		return err
	}

	return nil
}

func CacheDirectory(dirs ...string) string {
	return egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "golang", filepath.Join(dirs...))
}

func CacheBuildDirectory() string {
	return CacheDirectory("build")
}

func CacheModuleDirectory() string {
	return CacheDirectory("mod")
}

// attempt to build the golang environment that sets up
// the golang environment for caching.
func env() ([]string, error) {
	return envx.Build().FromEnv(os.Environ()...).
		Var("GOCACHE", CacheBuildDirectory()).
		Var("GOMODCACHE", CacheModuleDirectory()).
		Var("XDG_CACHE_HOME", envx.String(CacheDirectory(), "XDG_CACHE_HOME")). // used by go module code. only set it if its unset.
		Environ()
}

// attempt to build the golang environment that sets up
// the golang environment for caching.
func Env() []string {
	return errorsx.Must(env())
}

// Create a shell runtime that properly
// sets up the golang environment for caching.
func Runtime() shell.Command {
	return shell.Runtime().
		EnvironFrom(
			Env()...,
		)
}

func coveragedir() string {
	return egenv.EphemeralDirectory(".eg.coverage")
}
