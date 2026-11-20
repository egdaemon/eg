//go:build linux

package ttyx

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAttach(t *testing.T) {
	t.Run("no terminal", func(t *testing.T) {
		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer r.Close()
		defer w.Close()

		require.ErrorIs(t, attach(t.Context(), filepath.Join(t.TempDir(), "h"), r, w), ErrNoTerminal)
	})

	t.Run("keys and output", func(t *testing.T) {
		sock := filepath.Join(t.TempDir(), "h")
		h, err := NewHost(t.Context(), sock, nil)
		if err != nil {
			t.Skip("no pseudo terminal available", err)
		}
		defer h.Close()

		cmd := exec.Command("cat")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = h.Child(), h.Child(), h.Child()
		require.NoError(t, cmd.Start())
		defer cmd.Process.Kill()

		// the terminal of the client: what is typed on it is written to the parent side.
		parent, child := testpty(t)
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- attach(ctx, sock, child, child) }()
		defer func() {
			// the terminal is closed when the test ends, nothing may still be using it.
			cancel()
			<-done
		}()

		_, err = parent.Write([]byte("ping\n"))
		require.NoError(t, err)

		var seen []byte
		buf := make([]byte, 4096)
		require.NoError(t, parent.SetReadDeadline(time.Now().Add(10*time.Second)))
		for !bytes.Contains(seen, []byte("ping")) {
			n, err := parent.Read(buf)
			require.NoError(t, err)
			seen = append(seen, buf[:n]...)
		}
	})

	t.Run("leaving", func(t *testing.T) {
		sock := filepath.Join(t.TempDir(), "h")
		h, err := NewHost(t.Context(), sock, nil)
		if err != nil {
			t.Skip("no pseudo terminal available", err)
		}
		defer h.Close()

		cmd := exec.Command("cat")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = h.Child(), h.Child(), h.Child()
		require.NoError(t, cmd.Start())
		defer cmd.Process.Kill()

		_, child := testpty(t)
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- attach(ctx, sock, child, child) }()

		time.Sleep(200 * time.Millisecond)
		cancel()

		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("attach did not return when its context ended")
		}

		// leaving left the hosted command running.
		require.NoError(t, cmd.Process.Signal(syscall.Signal(0)))
	})

	t.Run("host ends", func(t *testing.T) {
		sock := filepath.Join(t.TempDir(), "h")
		h, err := NewHost(t.Context(), sock, nil)
		if err != nil {
			t.Skip("no pseudo terminal available", err)
		}

		_, child := testpty(t)
		done := make(chan error, 1)
		go func() { done <- attach(t.Context(), sock, child, child) }()

		time.Sleep(200 * time.Millisecond)
		require.NoError(t, h.Close())

		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("attach did not return when the host ended")
		}
	})
}
