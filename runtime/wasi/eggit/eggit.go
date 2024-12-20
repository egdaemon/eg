// Package eggit provides functionality around git and assumes that the git command is available.
package eggit

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

type hash [20]byte

// NewHash return a new Hash from a hexadecimal hash representation
func nhash(s string) hash {
	b, _ := hex.DecodeString(s)

	var h hash
	copy(h[:], b)

	return h
}

func (h hash) IsZero() bool {
	var empty hash
	return h == empty
}

func (h hash) String() string {
	return hex.EncodeToString(h[:])
}

type signature struct {
	Name  string
	Email string
	When  time.Time
}

type commit struct {
	// Hash of the commit object.
	Hash hash
	// Author is the original author of the commit.
	Author signature
	// Committer is the one performing the commit, might be different from
	// Author.
	Committer signature
}

// retrieve the commit metadata from from the environment.
func EnvCommit() *commit {
	return &commit{
		Hash: nhash(os.Getenv(_eg.EnvGitHeadCommit)),
		Committer: signature{
			Name:  os.Getenv(_eg.EnvGitHeadCommitAuthor),
			Email: os.Getenv(_eg.EnvGitHeadCommitEmail),
			When:  errorsx.Zero(time.Parse(time.RFC3339, os.Getenv(_eg.EnvGitHeadCommitTimestamp))),
		},
	}
}

// determine the commit hash for the given name.
func Commitish(ctx context.Context, treeish string) string {
	return ffigit.Commitish(ctx, treeish)
}

// clone a git repository from the given uri, remote and treeish.
func Clone(ctx context.Context, uri, remote, commit string) error {
	if strings.TrimSpace(uri) == "" {
		return fmt.Errorf("unable to clone url not specified: %s", uri)
	}

	if strings.TrimSpace(remote) == "" {
		return fmt.Errorf("unable to clone remote not specified: %s", remote)
	}

	return ffigit.Clone(ctx, uri, remote, commit)
}

// clone the repository specified by the eg environment variables into the working directory.
func AutoClone(ctx context.Context, _ eg.Op) error {
	return Clone(ctx, env.String("", "EG_GIT_HEAD_URI"), env.String("origin", "EG_GIT_HEAD_REMOTE"), env.String("main", "EG_GIT_HEAD_REF"))
}

type modified struct {
	init    sync.Once
	changed []string
}

func (t *modified) detect(ctx context.Context) error {
	var (
		path = egenv.RuntimeDirectory("eg.git.mod")
	)

	hcommit := envx.String("", _eg.EnvGitHeadCommit)
	bcommit := envx.String(hcommit, _eg.EnvGitBaseCommit)
	if stringsx.Blank(hcommit) {
		log.Println(errorsx.Errorf("environment variable %s is empty", _eg.EnvGitHeadCommit))
	}
	if stringsx.Blank(bcommit) {
		log.Println(errorsx.Errorf("environment variable %s is empty", _eg.EnvGitBaseCommit))
	}

	if hcommit == bcommit {
		// this is the where we compare a single commit.
		// just assume everything changed.
		return nil
	} else {
		err := shell.Run(
			ctx,
			shell.Newf(
				"git diff --name-only %s..%s | tee %s", bcommit, hcommit, path,
			).Directory(egenv.RootDirectory()),
			shell.Newf("cat %s", path),
		)
		if err != nil {
			return errorsx.Wrap(err, "unable to determine modified paths")
		}
	}

	mods, err := os.Open(path)
	if err != nil {
		return errorsx.Wrap(err, "unable to open mods")
	}

	t.changed = strings.Split(iox.String(mods), "\n")
	return nil
}

// check if the provided paths have any changes.
func (t *modified) Changed(paths ...string) func(context.Context) bool {
	return func(ctx context.Context) bool {
		t.init.Do(func() {
			errorsx.Log(t.detect(ctx))
		})

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

// used to check what directories have changed between the base and current commit.
func NewModified() modified {
	return modified{init: sync.Once{}}
}

// ensures the workspace has not been modified. useful for detecting
// if there have been changes during the run.
func Pristine() eg.OpFn {
	return func(ctx context.Context, _ eg.Op) error {
		return shell.Run(ctx, shell.New("git diff-index --quiet HEAD || (echo \"repository is dirty, aborting\" && false)"))
	}
}
