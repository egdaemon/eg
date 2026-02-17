// Package egdart has supporting functions for configuring the environment for running dart with caching.
package egdart

import (
	"context"
	"fmt"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/contextx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func CacheDirectory(dirs ...string) string {
	return egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "dart", filepath.Join(dirs...))
}

// attempt to build the dart environment that sets up
// the dart environment for caching.
func env() ([]string, error) {
	return envx.Build().
		Var("PUB_CACHE", CacheDirectory("pub-cache")).
		Environ()
}

// attempt to build the dart environment that sets up
// the dart environment for caching.
func Env() []string {
	return errorsx.Must(env())
}

// Create a shell runtime that properly
// sets up the dart environment for caching.
func Runtime() shell.Command {
	return shell.Runtime().
		EnvironFrom(
			Env()...,
		)
}

func TestOption() toption {
	return toption(nil)
}

type testOption struct {
	timeout time.Duration
	verbose string
}

type toption func(*testOption)

// provide a timeout for the command.
func (toption) Timeout(d time.Duration) toption {
	return func(o *testOption) {
		o.timeout = d
	}
}

// enable verbose output using expanded reporter.
func (toption) Verbose(b bool) toption {
	return func(o *testOption) {
		if b {
			o.verbose = "--reporter expanded"
		} else {
			o.verbose = ""
		}
	}
}

func (t testOption) options() (dst []string) {
	ignoreEmpty := func(dst []string, o string) []string {
		if stringsx.Blank(o) {
			return dst
		}

		return append(dst, o)
	}

	dst = ignoreEmpty(dst, t.verbose)

	return dst
}

// AutoDownload finds pubspec.yaml files and runs dart pub get in each project directory.
func AutoDownload() eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			denv []string
		)

		if denv, err = env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(denv...)

		for pubspec := range FindRoots(egenv.WorkingDirectory()) {
			cmd := stringsx.Join(" ", "dart", "pub", "get")
			if err := shell.Run(ctx, runtime.New(cmd).Directory(filepath.Dir(pubspec))); err != nil {
				return errorsx.Wrap(err, "unable to get dependencies")
			}
		}

		return nil
	})
}

// AutoTest finds pubspec.yaml files and runs dart test in each project directory.
func AutoTest(options ...toption) eg.OpFn {
	var (
		opts testOption
	)

	opts = langx.Clone(opts, options...)
	flags := stringsx.Join(" ", opts.options()...)

	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			denv []string
		)

		if denv, err = env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(denv...)
		timeout := timex.DurationMin(contextx.Until(ctx), timex.DurationFirstNonZero(opts.timeout, shell.DefaultTimeout))

		for pubspec := range FindRoots(egenv.WorkingDirectory()) {
			cmd := stringsx.Join(" ", "dart", "test", fmt.Sprintf("--timeout=%s", timeout), flags)
			if err := shell.Run(ctx, runtime.New(cmd).Directory(filepath.Dir(pubspec)).Timeout(timeout+time.Second)); err != nil {
				return errorsx.Wrap(err, "unable to run tests")
			}
		}

		return nil
	})
}

// AutoAnalyze finds pubspec.yaml files and runs dart analyze in each project directory.
func AutoAnalyze() eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) (err error) {
		var (
			denv []string
		)

		if denv, err = env(); err != nil {
			return err
		}

		runtime := shell.Runtime().EnvironFrom(denv...)

		for pubspec := range FindRoots(egenv.WorkingDirectory()) {
			cmd := stringsx.Join(" ", "dart", "analyze")
			if err := shell.Run(ctx, runtime.New(cmd).Directory(filepath.Dir(pubspec))); err != nil {
				return errorsx.Wrap(err, "unable to run analysis")
			}
		}

		return nil
	})
}

func FindRoots(root string) iter.Seq[string] {
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

			if filepath.Base(path) != "pubspec.yaml" {
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
