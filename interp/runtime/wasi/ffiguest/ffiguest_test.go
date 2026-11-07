package ffiguest_test

import (
	"errors"
	"syscall"
	"testing"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
	"github.com/stretchr/testify/require"
)

func TestError(t *testing.T) {
	t.Run("zero is no error", func(t *testing.T) {
		require.NoError(t, ffiguest.Error(0, errors.New("boom")))
	})

	t.Run("the code is a syscall.Errno in the chain", func(t *testing.T) {
		msg := errors.New("boom")
		err := ffiguest.Error(3, msg)
		errno, ok := errors.AsType[syscall.Errno](err)
		require.True(t, ok)
		require.Equal(t, syscall.Errno(3), errno)
		require.ErrorIs(t, err, msg)
		require.EqualError(t, err, "wasi host error: 3: boom")
	})

	t.Run("a missing message reports the code", func(t *testing.T) {
		errno, ok := errors.AsType[syscall.Errno](ffiguest.Error(3, nil))
		require.True(t, ok)
		require.Equal(t, syscall.Errno(3), errno)
	})

	t.Run("unrecoverable keeps its code", func(t *testing.T) {
		err := ffiguest.Error(ffierrors.ErrUnrecoverable, errors.New("boom"))
		require.ErrorAs(t, err, new(errorsx.Unrecoverable))
		errno, ok := errors.AsType[syscall.Errno](err)
		require.True(t, ok)
		require.Equal(t, syscall.Errno(ffierrors.ErrUnrecoverable), errno)
	})
}
