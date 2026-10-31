package eggit

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

type modified struct {
	o       sync.Once
	runtime shell.Command
	getenv  func(string) string
	workdir func(...string) string
	rundir  func(...string) string
	changed []string
}

func (t *modified) detect(ctx context.Context) error {
	env := envx.NewEnviron(t.getenv)
	var (
		path = t.rundir("eg.git.mod")
	)

	hcommit := env.String("", _eg.EnvGitHeadCommit)
	bcommit := env.String(hcommit, _eg.EnvGitBaseCommit)
	if strings.TrimSpace(hcommit) == "" {
		log.Println(fmt.Errorf("environment variable %s is empty", _eg.EnvGitHeadCommit))
	} else {
		debugx.Println("git.modified head commit", hcommit)
	}
	if strings.TrimSpace(bcommit) == "" {
		log.Println(fmt.Errorf("environment variable %s is empty", _eg.EnvGitBaseCommit))
	} else {
		debugx.Println("git.modified base commit", bcommit)
	}

	if hcommit == bcommit {
		return nil
	}

	// handle detecting missing commits
	// if we can't find one of the commits
	// then we treat it as if the commits were identical
	// resulting everything being treated as changed.
	err := shell.Run(
		ctx,
		t.runtime.Newf(
			"git show --name-only %s > /dev/null 2>&1", bcommit,
		).Directory(t.workdir()),
		t.runtime.Newf(
			"git show --name-only %s > /dev/null 2>&1", hcommit,
		).Directory(t.workdir()),
	)
	if err != nil {
		return nil
	}

	err = shell.Run(
		ctx,
		t.runtime.Newf(
			"git diff --name-only %s..%s | tee %s > /dev/null 2>&1", bcommit, hcommit, path,
		).Directory(t.workdir()),
	)
	if err != nil {
		return errorsx.Wrap(err, "git diff failed")
	}

	mods, err := os.Open(path)
	// if we didnt generate the changeset then no changes occurred.
	if fsx.ErrIsNotExist(err) != nil {
		return nil
	} else if err != nil {
		return err
	}

	smods := iox.String(mods)

	if stringsx.Blank(smods) {
		return nil
	}

	t.changed = strings.Split(smods, "\n")
	return nil
}

func (t *modified) Changed(paths ...string) func(context.Context) bool {
	return func(ctx context.Context) bool {
		t.o.Do(func() {
			errorsx.Log(t.detect(ctx))
		})

		debugx.Println("changed", len(t.changed), t.changed, "paths", len(paths))
		if len(t.changed) == 0 || len(paths) == 0 {
			return true
		}

		return stringsx.Present(slicesx.FindOrZero(func(s string) bool {
			for _, n := range paths {
				if strings.HasPrefix(s, n) {
					return true
				}
			}

			return false
		}, t.changed...))
	}
}

type modifiedoption func(*modified)

// ModifiedOptionRuntime overrides the shell runtime used to run git commands.
func ModifiedOptionRuntime(r shell.Command) modifiedoption {
	return func(m *modified) {
		m.runtime = r
	}
}

// ModifiedOptionGetenv overrides how environment variables are looked up.
func ModifiedOptionGetenv(fn func(string) string) modifiedoption {
	return func(m *modified) {
		m.getenv = fn
	}
}

// ModifiedOptionWorkingDirectory overrides the working directory git commands are run from.
func ModifiedOptionWorkingDirectory(dir string) modifiedoption {
	return func(m *modified) {
		m.workdir = func(paths ...string) string {
			return filepath.Join(dir, filepath.Join(paths...))
		}
	}
}

// ModifiedOptionRuntimeDirectory overrides the directory the changeset is written to.
func ModifiedOptionRuntimeDirectory(dir string) modifiedoption {
	return func(m *modified) {
		m.rundir = func(paths ...string) string {
			return filepath.Join(dir, filepath.Join(paths...))
		}
	}
}

func NewModified(options ...modifiedoption) modified {
	cfg := modified{
		runtime: shell.Runtime(),
		getenv:  os.Getenv,
		workdir: egenv.DirectoryWorking,
		rundir:  egunsafe.RuntimeDirectory,
	}

	for _, opt := range options {
		opt(&cfg)
	}

	return modified{o: sync.Once{}, runtime: cfg.runtime, getenv: cfg.getenv, workdir: cfg.workdir, rundir: cfg.rundir}
}
