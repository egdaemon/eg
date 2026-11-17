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
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
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
		debugx.Println("git.modified skipping detection: base and head commits are identical, no changeset to compute, everything is treated as changed", hcommit)
		return nil
	}

	path := t.rundir(fmt.Sprintf("eg.git.%s.mod", md5x.FormatHex(md5x.Digest(bcommit, hcommit))))
	debugx.Println("git.modified detecting", bcommit+".."+hcommit, "workdir", t.workdir(), "changeset", path)

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
		debugx.Println("git.modified skipping detection: base or head commit is not present in the repository, everything is treated as changed", err)
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
		debugx.Println("git.modified changeset file was not generated, treating as no changes", path)
		return nil
	} else if err != nil {
		return err
	}

	smods := iox.String(mods)

	if stringsx.Blank(smods) {
		debugx.Println("git.modified changeset is empty, treating as no changes", path)
		return nil
	}

	t.changed = strings.Split(smods, "\n")
	debugx.Println("git.modified changeset loaded", len(t.changed), "files from", path)
	return nil
}

func (t *modified) Changed(paths ...string) func(context.Context) bool {
	return func(ctx context.Context) bool {
		t.o.Do(func() {
			errorsx.Log(t.detect(ctx))
		})

		debugx.Println("changed", len(t.changed), t.changed, "paths", len(paths))
		if len(t.changed) == 0 {
			debugx.Println("git.modified Changed: no known changeset, reporting changed")
			return true
		}

		if len(paths) == 0 {
			debugx.Println("git.modified Changed: no paths to filter by, reporting changed")
			return true
		}

		match := slicesx.FindOrZero(func(s string) bool {
			for _, n := range paths {
				if strings.HasPrefix(s, n) {
					return true
				}
			}

			return false
		}, t.changed...)

		if stringsx.Present(match) {
			debugx.Println("git.modified Changed: changed file matched watched paths, reporting changed", match, paths)
			return true
		}

		debugx.Println("git.modified Changed: no changed file matched watched paths, reporting unchanged", paths)
		return false
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
	cfg := langx.Clone(modified{
		runtime: shell.Runtime(),
		getenv:  os.Getenv,
		workdir: egenv.DirectoryWorking,
		rundir:  egenv.DirectoryTemp,
	}, options...)
	return modified{o: sync.Once{}, runtime: cfg.runtime, getenv: cfg.getenv, workdir: cfg.workdir, rundir: cfg.rundir}
}
