package eggit

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/ffigit"
	"github.com/egdaemon/eg/runtime/wasi/env"
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
		Hash: nhash(os.Getenv("EG_GIT_HEAD_COMMIT")),
		Committer: signature{
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
