package m1

import (
	"context"

	_ "github.com/egdaemon/eg/runtime/autowasinet"
	"github.com/egdaemon/eg/runtime/wasi/eg"
)

func Root(ctx context.Context, op eg.Op) error {
	value := "nested"
	_ = value
	return eg.Perform(
		ctx, eg.UnsafeRunner(ctx, eg.DefaultModule().ToModuleRunner(), `m1/m1.wasm.d/module.14.3.wasm`),
	)
}
