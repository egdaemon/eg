package ffierrors

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"syscall"
)

func ErrnoSuccess() syscall.Errno {
	return syscall.Errno(0x0)
}

// convert a syscall.Errno into an idiomatic golang error.
func Error(cause syscall.Errno) error {
	if cause == ErrnoSuccess() {
		return nil
	}
	return cause
}

func Errno(err error) syscall.Errno {
	if err == nil {
		return ErrnoSuccess()
	}

	if errno, ok := err.(syscall.Errno); ok {
		return errno
	}

	return makeErrnoSlow(err)
}

func makeErrnoSlow(err error) (ret syscall.Errno) {
	var timeout interface{ Timeout() bool }
	if errors.As(err, &ret) {
		return ret
	}
	switch {
	case errors.Is(err, context.Canceled):
		return syscall.ECANCELED
	case errors.Is(err, context.DeadlineExceeded):
		return syscall.ETIMEDOUT
	case errors.Is(err, io.ErrUnexpectedEOF),
		errors.Is(err, fs.ErrClosed),
		errors.Is(err, net.ErrClosed):
		return syscall.EIO
	}

	if errors.As(err, &timeout) {
		if timeout.Timeout() {
			return syscall.ETIMEDOUT
		}
	}

	panic(fmt.Errorf("unexpected error: %v", err))
}
