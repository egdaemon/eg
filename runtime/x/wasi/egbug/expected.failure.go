package egbug

import (
	"context"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
)

const (
	ErrUnexpectedSuccessfulOperation = errorsx.String("expected an error but operation succeeded")
)

// for commands we want to fail in a particular manner. useful for tests.
func ExpectedFailure(op eg.OpFn, allowed ...error) eg.OpFn {
	return func(ctx context.Context, o eg.Op) error {
		if err := op(ctx, o); err == nil {
			return ErrUnexpectedSuccessfulOperation
		} else if errorsx.Ignore(err, allowed...) != nil {
			return errorsx.Wrap(err, "an unexpected error occurred")
		}

		return nil
	}
}
