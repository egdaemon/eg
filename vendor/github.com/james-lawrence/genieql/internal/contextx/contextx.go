package contextx

import (
	"context"
	"errors"
)

func IsCancelled(err error) bool {
	return errors.Is(err, context.Canceled)
}

func IgnoreDeadlineExceeded(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return err
}
