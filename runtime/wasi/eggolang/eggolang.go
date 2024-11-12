package eggolang

import (
	"context"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
)

func AutoCompile() eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) error {
		return errorsx.New("unimplemented")
	})
}

func AutoTest() eg.OpFn {
	return eg.OpFn(func(ctx context.Context, _ eg.Op) error {
		return errorsx.New("unimplemented")
	})
}
