//go:build !wasip1 && !darwin && !linux && !windows

package wnetruntime

// ensure we can at least compile on unknown platforms.

import (
	"context"
	"syscall"
)

func (t network) Open(ctx context.Context, af, socktype, protocol int) (fd int, err error) {
	return -1, syscall.ENOTSUP
}

func (t network) SetSocketOption(ctx context.Context, fd int, level, name int, value []byte) error {
	return syscall.ENOTSUP
}

func (t network) GetSocketOption(ctx context.Context, fd int, level, name int, value []byte) (any, error) {
	return nil, syscall.ENOTSUP
}
