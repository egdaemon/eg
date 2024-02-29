package eggit

import (
	"context"

	"github.com/egdaemon/eg/runtime/wasi/internal/ffigit"
)

func Commitish(ctx context.Context, treeish string) string {
	return ffigit.Commitish(ctx, treeish)
}
