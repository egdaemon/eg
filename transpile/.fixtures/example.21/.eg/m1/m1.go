package m1

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
)

func Root(ctx context.Context, op eg.Op) error {
	value := "nested"
	return eg.Perform(
		ctx,
		eg.Module(ctx, eg.DefaultModule(), func(ctx context.Context, op eg.Op) error {
			log.Println(value)
			return nil
		}),
	)
}
