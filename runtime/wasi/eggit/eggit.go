package eggit

import (
	"context"
	"fmt"
	"strings"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/env"
	"github.com/egdaemon/eg/runtime/wasi/internal/ffigit"
)

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
	return Clone(ctx, env.String("", "EG_GIT_URI"), env.String("origin", "EG_GIT_REMOTE"), env.String("main", "EG_GIT_REF"))
}
