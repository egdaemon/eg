package eggit

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
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

func Commitish(ctx context.Context, treeish string) string {
	return ffigit.Commitish(ctx, treeish)
}

func Clone(ctx context.Context, uri, remote, branch string) error {
	if strings.TrimSpace(uri) == "" {
		return fmt.Errorf("unable to clone url not specified: %s", uri)
	}

	if strings.TrimSpace(remote) == "" {
		return fmt.Errorf("unable to clone remote not specified: %s", remote)
	}

	return ffigit.Clone(ctx, uri, remote, branch)
}

func AutoClone(ctx context.Context, _ eg.Op) error {
	return Clone(ctx, env.String("", "EG_GIT_HEAD_URI"), env.String("origin", "EG_GIT_HEAD_REMOTE"), env.String("main", "EG_GIT_HEAD_REF"))
}

type modified struct {
	changed []string
}

func (t modified) Changed(paths ...string) bool {
	if len(t.changed) == 0 {
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

func NewModified(ctx context.Context) (modified, error) {
	hcommit := envx.String("", _eg.EnvGitHeadCommit)
	bcommit := envx.String(hcommit, _eg.EnvGitBaseCommit)
	if stringsx.Blank(hcommit) {
		log.Println(errorsx.Errorf("environment variable %s is empty", _eg.EnvGitHeadCommit))
	}
	if stringsx.Blank(bcommit) {
		log.Println(errorsx.Errorf("environment variable %s is empty", _eg.EnvGitBaseCommit))
	}

	err := shell.Run(
		ctx,
		shell.Newf("ls -lha /opt"),
		shell.Newf("git diff --name-only %s..%s | tee %s", bcommit, hcommit, egenv.EphemeralDirectory("eg.git.mod")),
		shell.Newf("cat %s", egenv.EphemeralDirectory("eg.git.mod")),
	)
	if err != nil {
		return modified{}, errorsx.Wrap(err, "unable to determine modified paths")
	}

	fsx.PrintDir(os.DirFS(egenv.EphemeralDirectory()))
	mods, err := os.Open(egenv.EphemeralDirectory("eg.git.mod"))
	if err != nil {
		return modified{}, errorsx.Wrap(err, "unable to open mods")
	}
	changed := iox.String(mods)
	return modified{changed: strings.Split(changed, "\n")}, nil
}
