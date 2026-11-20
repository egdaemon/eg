//go:build linux

package ttyx

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestHost(t *testing.T) {
	t.Run("input", func(t *testing.T) {
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

		a, err := net.Dial("unix", sock)
		require.NoError(t, err)
		defer a.Close()
		b, err := net.Dial("unix", sock)
		require.NoError(t, err)
		defer b.Close()

		// what a client types reaches the command, and what it prints reaches every client.
		require.NoError(t, writeFrame(a, frameData, []byte("hello\n")))
		for _, conn := range []net.Conn{a, b} {
			var seen []byte
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
			for !bytes.Contains(seen, []byte("hello")) {
				kind, payload, err := readFrame(conn)
				require.NoError(t, err)
				require.Equal(t, frameData, kind)
				seen = append(seen, payload...)
			}
		}
	})

	t.Run("replay", func(t *testing.T) {
		dir := t.TempDir()
		sock := filepath.Join(dir, "h")
		mirror, err := os.Create(filepath.Join(dir, "log"))
		require.NoError(t, err)
		defer mirror.Close()

		h, err := NewHost(t.Context(), sock, mirror)
		if err != nil {
			t.Skip("no pseudo terminal available", err)
		}
		defer h.Close()

		cmd := exec.Command("sh", "-c", "echo before; cat")
		cmd.Stdin, cmd.Stdout, cmd.Stderr = h.Child(), h.Child(), h.Child()
		require.NoError(t, cmd.Start())
		defer cmd.Process.Kill()

		// nobody is attached while it prints: the mirror has it all the same.
		require.Eventually(t, func() bool {
			raw, err := os.ReadFile(filepath.Join(dir, "log"))
			return err == nil && bytes.Contains(raw, []byte("before"))
		}, 10*time.Second, 20*time.Millisecond)

		late, err := net.Dial("unix", sock)
		require.NoError(t, err)
		defer late.Close()

		// a terminal reset first, then what was printed.
		require.NoError(t, late.SetReadDeadline(time.Now().Add(10*time.Second)))
		kind, payload, err := readFrame(late)
		require.NoError(t, err)
		require.Equal(t, frameData, kind)
		require.Equal(t, []byte("\x1bc"), payload)

		var seen []byte
		for !bytes.Contains(seen, []byte("before")) {
			_, payload, err := readFrame(late)
			require.NoError(t, err)
			seen = append(seen, payload...)
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

		// one client leaves abruptly, mid frame, another politely.
		rude, err := net.Dial("unix", sock)
		require.NoError(t, err)
		_, err = rude.Write([]byte{frameData, 0, 0, 0, 9, 'x'})
		require.NoError(t, err)
		require.NoError(t, rude.Close())

		polite, err := net.Dial("unix", sock)
		require.NoError(t, err)
		require.NoError(t, writeFrame(polite, frameData, []byte("bye\n")))
		require.NoError(t, polite.Close())

		time.Sleep(200 * time.Millisecond)
		require.NoError(t, cmd.Process.Signal(syscall.Signal(0)))

		// the command still gets input from whoever attaches next.
		next, err := net.Dial("unix", sock)
		require.NoError(t, err)
		defer next.Close()
		require.NoError(t, writeFrame(next, frameData, []byte("again\n")))

		var seen []byte
		require.NoError(t, next.SetReadDeadline(time.Now().Add(10*time.Second)))
		for !bytes.Contains(seen, []byte("again")) {
			_, payload, err := readFrame(next)
			require.NoError(t, err)
			seen = append(seen, payload...)
		}
	})

	t.Run("resize", func(t *testing.T) {
		sock := filepath.Join(t.TempDir(), "h")
		h, err := NewHost(t.Context(), sock, nil)
		if err != nil {
			t.Skip("no pseudo terminal available", err)
		}
		defer h.Close()

		c, err := net.Dial("unix", sock)
		require.NoError(t, err)
		defer c.Close()
		require.NoError(t, writeFrame(c, frameResize, encodeSize(24, 80)))

		require.Eventually(t, func() bool {
			var ws *unix.Winsize
			if err := control(h.Child(), func(fd int) (err error) {
				ws, err = unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
				return err
			}); err != nil {
				return false
			}
			return ws.Row == 24 && ws.Col == 80
		}, 10*time.Second, 20*time.Millisecond)
	})

	t.Run("nil", func(t *testing.T) {
		// a run with no hosted terminal carries a nil host, which has no terminal, no socket and nothing to close.
		var h *Host
		require.Nil(t, h.Child())
		require.Equal(t, "", h.Path())
		require.NoError(t, h.Close())
	})

	t.Run("close", func(t *testing.T) {
		sock := filepath.Join(t.TempDir(), "h")
		h, err := NewHost(t.Context(), sock, nil)
		if err != nil {
			t.Skip("no pseudo terminal available", err)
		}

		c, err := net.Dial("unix", sock)
		require.NoError(t, err)
		defer c.Close()

		require.NoError(t, h.Close())
		_, err = os.Stat(sock)
		require.True(t, os.IsNotExist(err))
		_, err = net.Dial("unix", sock)
		require.Error(t, err)
	})
}
