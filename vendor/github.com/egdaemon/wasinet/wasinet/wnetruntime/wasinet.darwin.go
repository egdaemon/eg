//go:build !wasip1 && darwin

package wnetruntime

import (
	"context"
	"fmt"
	"syscall"

	"github.com/egdaemon/wasinet/wasinet/ffi"
	"github.com/egdaemon/wasinet/wasinet/internal/errorsx"
	"golang.org/x/sys/unix"
)

func (t network) Open(ctx context.Context, af, socktype, protocol int) (fd int, err error) {
	fd, err = unix.Socket(af, socktype, protocol)
	if err != nil {
		return fd, err
	}
	defer func() {
		if err == nil {
			return
		}

		unix.Close(fd)
	}()

	// syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC are required by golang's runtime for the pollfd to operate correctly.
	// as a result we unconditionally set them here.

	if err := unix.SetNonblock(fd, true); err != nil {
		return -1, fmt.Errorf("failed to set non-blocking: %w", err)
	}

	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
	if err != nil {
		return -1, fmt.Errorf("failed to get FD flags for CLOEXEC: %w", err)
	}
	if _, err := unix.FcntlInt(uintptr(fd), unix.F_SETFD, flags|unix.FD_CLOEXEC); err != nil {
		return -1, fmt.Errorf("failed to set CLOEXEC: %w", err)
	}

	return fd, nil
}

func (t network) SetSocketOption(ctx context.Context, fd int, level, name int, value []byte) error {
	switch name {
	case syscall.SO_LINGER, syscall.SO_RCVTIMEO, syscall.SO_SNDTIMEO:
		v := &unix.Timeval{}
		vptr, vlen := ffi.Slice(value)
		tvptr, _ := ffi.Pointer(v)
		if err := ffi.RawRead(ffi.Native{}, ffi.Native{}, tvptr, vptr, vlen); err != nil {
			return err
		}
		errno := unix.SetsockoptTimeval(fd, level, name, v)
		// slog.Log(ctx, slog.LevelDebug, "sock_setsockopt_timeval", slog.Int("fd", fd), slog.Int("level", level), slog.Int("name", name), slog.Any("value", v), slog.Int("errno", int(ffierrors.Errno(errno))))
		return errno
	default:
		value := errorsx.Must(ffi.Uint32ReadNative(ffi.Slice(value)))
		// slog.Log(ctx, slog.LevelDebug, "sock_setsockopt_int", slog.Int("fd", fd), slog.Int("level", level), slog.Int("name", name), slog.Uint64("value", uint64(value)))
		return unix.SetsockoptInt(fd, level, name, int(value))
	}
}

func (t network) GetSocketOption(ctx context.Context, fd int, level, name int, value []byte) (any, error) {
	switch name {
	case syscall.SO_LINGER:
		return unix.Timeval{}, syscall.ENOTSUP
	default:
		return unix.GetsockoptInt(int(fd), int(level), int(name))
	}
}
