//go:build linux

package ttyx

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRead(t *testing.T) {
	t.Run("drains piped input", func(t *testing.T) {
		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer r.Close()
		stdin = func() *os.File { return r }

		_, err = w.Write([]byte("piped\ninput"))
		require.NoError(t, err)
		w.Close()

		in := Read(t.Context())
		defer in.Close()
		got, err := io.ReadAll(in)
		require.NoError(t, err)
		require.Equal(t, "piped\ninput", string(got))
	})

	t.Run("a terminal is not read", func(t *testing.T) {
		parent, child := testpty(t)
		stdin = func() *os.File { return child }

		// the user has typed something, draining stdin would consume it or block.
		_, err := parent.Write([]byte("typed\n"))
		require.NoError(t, err)

		in := Read(t.Context())
		defer in.Close()
		got, err := io.ReadAll(in)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("nothing on stdin is empty", func(t *testing.T) {
		null, err := os.Open(os.DevNull)
		require.NoError(t, err)
		defer null.Close()
		stdin = func() *os.File { return null }

		in := Read(t.Context())
		defer in.Close()
		got, err := io.ReadAll(in)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("stops with the context", func(t *testing.T) {
		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer r.Close()
		defer w.Close()
		stdin = func() *os.File { return r }

		ctx, cancel := context.WithCancel(t.Context())
		in := Read(ctx)
		defer in.Close()

		done := make(chan error, 1)
		go func() {
			_, err := io.ReadAll(in)
			done <- err
		}()

		cancel()
		select {
		case err := <-done:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(5 * time.Second):
			t.Fatal("the read did not stop when the context was cancelled")
		}
	})

	t.Run("close stops a read waiting on stdin", func(t *testing.T) {
		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer r.Close()
		defer w.Close()
		stdin = func() *os.File { return r }

		in := Read(t.Context())

		done := make(chan error, 1)
		go func() {
			_, err := io.ReadAll(in)
			done <- err
		}()

		require.NoError(t, in.Close())
		select {
		case err := <-done:
			require.ErrorIs(t, err, io.ErrClosedPipe)
		case <-time.After(5 * time.Second):
			t.Fatal("the read did not stop when the reader was closed")
		}
	})
}
