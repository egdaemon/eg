package eggit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/internal/ffigit"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Commit = object.Commit

func EnvCommit() *Commit {
	return &Commit{
		Hash: plumbing.NewHash(os.Getenv("EG_GIT_HEAD_COMMIT")),
		Committer: object.Signature{
			Name:  os.Getenv("EG_GIT_HEAD_COMMIT_AUTHOR"),
			Email: os.Getenv("EG_GIT_HEAD_COMMIT_EMAIL"),
			When:  errorsx.Zero(time.Parse(time.RFC3339, os.Getenv("EG_GIT_HEAD_COMMIT_TIMESTAMP"))),
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
