package eggolang

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/contextx"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/coverage/golangcov"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/modfilex"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

type config struct {
	flags     []string
	environ   []string
	bctx      build.Context
	timeout   time.Duration
	coverage  string // coverage mode, test only.
	count     string // test only.
	randomize string // test only.
	verbose   string // test only.
}

type option = func(*config)
type options []option

func New() options {
	return options(nil)
}

// provide a timeout for the command.
func (t options) Timeout(d time.Duration) options {
	return append(t, func(o *config) {
		o.timeout = d
	})
}

// enable debug flag
func (t options) Debug(b bool) options {
	return append(t, func(o *config) {
		if !b {
			return
		}
		o.flags = append(o.flags, "-x")
	})
}

func (t options) Tags(tags ...string) options {
	return append(t, func(o *config) {
		o.bctx.BuildTags = tags
	})
}

func (t options) WorkingDirectory(s string) options {
	return append(t, func(o *config) {
		o.bctx.Dir = s
	})
}

// escape hatch for setting command line flags.
// useful for flags not explicitly implemented by this package.
func (t options) Flags(flags ...string) options {
	return append(t, func(o *config) {
		o.flags = append(o.flags, flags...)
	})
}

// escape hatch for setting command environment variables.
// useful for flags not explicitly implemented by this package.
func (t options) Environ(envvars ...string) options {
	return append(t, func(o *config) {
		o.environ = append(o.environ, envvars...)
	})
}

// Randomize execution order of the tests using the provided seed value.
// defaults to on.
// 0 = off
// 1 = on
// N = seed
func (t options) Randomize(seed int) options {
	return append(t, func(o *config) {
		switch seed {
		case 0:
			o.randomize = "-shuffle off"
		case 1:
			o.randomize = "-shuffle on"
		default:
			o.randomize = fmt.Sprintf("-shuffle %d", seed)
		}
	})
}

// The number of times to run the test suite.
// 0 uses go test default.
func (t options) Count(n int) options {
	return append(t, func(o *config) {
		o.count = countflag(n)
	})
}

func countflag(n int) string {
	if n == 0 {
		return ""
	}

	return fmt.Sprintf("-count %d", n)
}

// used to disable golang test caching.
// short for Count(1) see go help test.
// will only be applied if count is unset.
func (t options) NoCache() options {
	return append(t, func(o *config) {
		if stringsx.Present(o.count) {
			return
		}

		o.count = countflag(1)
	})
}

func (t options) Verbose(b bool) options {
	return append(t, func(o *config) {
		if b {
			o.verbose = "-v"
		} else {
			o.verbose = ""
		}
	})
}

// flags shared by every go command.
func (t config) buildflags() (opts []string) {
	opts = append(opts, t.flags...)
	if len(t.bctx.BuildTags) > 0 {
		opts = append(opts, fmt.Sprintf("-tags=%s", strings.Join(t.bctx.BuildTags, ",")))
	}

	return opts
}

// flags for go test.
func (t config) testflags() (dst []string) {
	ignoreEmpty := func(dst []string, o string) []string {
		if stringsx.Blank(o) {
			return dst
		}

		return append(dst, o)
	}
	dst = t.buildflags()
	dst = ignoreEmpty(dst, t.randomize)
	dst = ignoreEmpty(dst, t.count)
	dst = ignoreEmpty(dst, t.verbose)
	dst = append(dst, t.coverage)

	return dst
}

// shorthand for New().AutoInstall()
func AutoInstall() eg.OpFn {
	return New().AutoInstall()
}

// shorthand for New().AutoCompile()
func AutoCompile() eg.OpFn {
	return New().AutoCompile()
}

// shorthand for New().AutoTest()
func AutoTest() eg.OpFn {
	return New().AutoTest()
}

func (t options) AutoInstall() eg.OpFn {
	return t.autoinstall(shell.Runtime())
}

func (t options) autoinstall(base shell.Command) eg.OpFn {
	opts := langx.Clone(config{}, t...)
	flags := stringsx.Join(" ", opts.buildflags()...)

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			goenv []string
		)

		if goenv, err = env(); err != nil {
			return err
		}

		runtime := base.EnvironFrom(goenv...).EnvironFrom(opts.environ...)
		timeout := timex.DurationMin(contextx.Until(ctx), timex.DurationFirstNonZero(opts.timeout, shell.DefaultTimeout))

		for gomod := range modfilex.FindModules(stringsx.DefaultIfBlank(opts.bctx.Dir, egenv.WorkingDirectory())) {
			cmd := stringsx.Join(" ", slicesx.Filter(func(s string) bool { return stringsx.Present(s) }, "go", "-C", filepath.Dir(gomod), "install", "-modcacherw", flags, "./...")...)
			if err := shell.Run(ctx, runtime.New(cmd).Timeout(timeout)); err != nil {
				return errorsx.Wrap(err, "unable to run tests")
			}
		}

		return nil
	})
}

func (t options) AutoCompile() eg.OpFn {
	return t.autocompile(shell.Runtime())
}

func (t options) autocompile(base shell.Command) eg.OpFn {
	opts := langx.Clone(config{}, t...)
	flags := stringsx.Join(" ", opts.buildflags()...)

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			goenv []string
		)

		if goenv, err = env(); err != nil {
			return err
		}

		runtime := base.EnvironFrom(goenv...).EnvironFrom(opts.environ...)
		timeout := timex.DurationMin(contextx.Until(ctx), timex.DurationFirstNonZero(opts.timeout, shell.DefaultTimeout))

		for gomod := range modfilex.FindModules(stringsx.DefaultIfBlank(opts.bctx.Dir, egenv.WorkingDirectory())) {
			cmd := stringsx.Join(" ", "go", "-C", filepath.Dir(gomod), "build", "-modcacherw", flags, "./...")
			if err := shell.Run(ctx, runtime.New(cmd).Timeout(timeout)); err != nil {
				return errorsx.Wrap(err, "unable to compile")
			}
		}

		return nil
	})
}

func (t options) AutoTest() eg.OpFn {
	return t.autotest(shell.Runtime())
}

func (t options) autotest(base shell.Command) eg.OpFn {
	opts := langx.Clone(config{coverage: "-covermode count"}, t...)
	flags := stringsx.Join(" ", opts.testflags()...)

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			goenv []string
		)

		covpath := coveragedir()
		if err := shell.Run(ctx, base.Newf("mkdir -p %s", covpath)); err != nil {
			return errorsx.Wrap(err, "unable to run tests")
		}

		if goenv, err = env(); err != nil {
			return err
		}

		runtime := base.EnvironFrom(goenv...).EnvironFrom(opts.environ...)
		timeout := timex.DurationMin(contextx.Until(ctx), timex.DurationFirstNonZero(opts.timeout, shell.DefaultTimeout))

		for gomod := range modfilex.FindModules(egenv.WorkingDirectory()) {
			cmd := stringsx.Join(" ", "go", "-C", filepath.Dir(gomod), "test", "-modcacherw", fmt.Sprintf("-timeout=%s", timeout), flags, fmt.Sprintf("-coverprofile %s", filepath.Join(covpath, md5x.String(gomod))), "./...")
			if err := shell.Run(ctx, runtime.New(cmd).Timeout(timeout+time.Second)); err != nil {
				return errorsx.Wrap(err, "unable to run tests")
			}
		}

		return nil
	})
}

// Coverage is the coverage of the profiles AutoTest wrote, with paths relative to the workspace. it is recorded with
// egcoverage.Report.
func Coverage() coverage.Options {
	return coverage.New(func(ctx context.Context) coverage.Seq {
		return golangcov.Coverage(ctx, coveragedir(), golangcov.Modules(modfilex.FindModules(egenv.WorkingDirectory())))
	}).Transforms(coverage.TransformPath(egenv.PathWorkspaceRelative))
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
	return envx.Build().FromEnviron(os.Environ()...).
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
