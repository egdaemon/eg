package ffierrors

import (
	"errors"
	"os"

	"github.com/james-lawrence/genieql/internal/errorsx"
)

const (
	ErrNotImplemented = 999
	ErrUnrecoverable  = 1000
)

func Exit(cause error) {
	var (
		unrecoverable errorsx.Unrecoverable
	)

	if errors.Is(cause, &unrecoverable) {
		os.Exit(ErrUnrecoverable)
	}

	os.Exit(1)
}

func Error(code uint32, msg error) error {
	if code == 0 {
		return nil
	}

	cause := errorsx.Wrapf(msg, "wasi host error: %d", code)
	switch code {
	case ErrUnrecoverable:
		return errorsx.NewUnrecoverable(cause)
	default:
		return cause
	}
}
