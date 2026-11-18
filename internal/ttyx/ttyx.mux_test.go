//go:build linux

package ttyx

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func termios(t *testing.T, f *os.File) *unix.Termios {
	t.Helper()
	var tio *unix.Termios
	require.NoError(t, control(f, func(fd int) (err error) {
		tio, err = unix.IoctlGetTermios(fd, unix.TCGETS)
		return err
	}))
	return tio
}

// TestRawStaircase reproduces the staircase regression from c616a90b: execAttach put the user's terminal in raw mode
// for the module, and everything else writing to that terminal at the same time, e.g.) other shell commands or the log,
// writes a bare \n. raw mode belongs to the terminal rather than the descriptor, so with output processing off that
// \n no longer returns to the first column and every line starts where the previous one ended.
func TestRawStaircase(t *testing.T) {
	parent, attached := testpty(t)

	restore, err := Raw(attached)
	require.NoError(t, err)
	defer restore()

	// a separate writer on the same terminal, e.g.) a parallel shell command given os.Stdout.
	other, err := os.OpenFile(attached.Name(), os.O_WRONLY|unix.O_NOCTTY, 0)
	require.NoError(t, err)
	defer other.Close()

	// the trailing . marks the end of the output, so the read never waits on bytes that will not come.
	_, err = other.Write([]byte("a\nb\n."))
	require.NoError(t, err)

	got, err := bufio.NewReader(parent).ReadString('.')
	require.NoError(t, err)
	require.Equal(t, "a\r\nb\r\n.", got, "every line written to the terminal must return to the first column")
}

func TestRaw(t *testing.T) {
	t.Run("input is raw and output is processed", func(t *testing.T) {
		_, child := testpty(t)
		restore, err := Raw(child)
		require.NoError(t, err)
		defer restore()

		tio := termios(t, child)
		require.NotZero(t, tio.Oflag&unix.OPOST)
		require.NotZero(t, tio.Oflag&unix.ONLCR)
		require.Zero(t, tio.Lflag&unix.ICANON)
		require.Zero(t, tio.Lflag&unix.ECHO)
		require.Zero(t, tio.Lflag&unix.ISIG)
	})

	t.Run("cbreak keeps signals", func(t *testing.T) {
		_, child := testpty(t)
		restore, err := cbreak(child)
		require.NoError(t, err)
		defer restore()

		tio := termios(t, child)
		require.NotZero(t, tio.Lflag&unix.ISIG)
		require.Zero(t, tio.Lflag&unix.ICANON)
	})

	for _, tc := range []struct {
		name  string
		first bool // the first hold is released first.
	}{
		{name: "overlapping holds released in order restore the terminal", first: true},
		{name: "overlapping holds released out of order restore the terminal", first: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, child := testpty(t)
			original := termios(t, child)

			// a second descriptor of the same device, as a second module session would have.
			second, err := os.OpenFile(child.Name(), os.O_RDWR|unix.O_NOCTTY, 0)
			require.NoError(t, err)
			defer second.Close()

			r1, err := Raw(child)
			require.NoError(t, err)
			r2, err := Raw(second)
			require.NoError(t, err)

			if tc.first {
				r1()
				require.Zero(t, termios(t, child).Lflag&unix.ICANON, "the terminal is still held")
				r2()
			} else {
				r2()
				require.Zero(t, termios(t, child).Lflag&unix.ICANON, "the terminal is still held")
				r1()
			}

			require.Equal(t, original, termios(t, child))
		})
	}
}

type syncbuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (t *syncbuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.Write(p)
}

func (t *syncbuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.String()
}

// testmux is a mux of a pseudo terminal standing in for the user's, keys is what the user types.
func testmux(t *testing.T) (m *Mux, keys io.Writer, out *syncbuffer) {
	t.Helper()
	_, user := testpty(t)
	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { w.Close() })

	ctl, err := os.OpenFile(user.Name(), os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)

	out = &syncbuffer{}
	m, err = newmux(t.Context(), ctl, r, out)
	require.NoError(t, err)
	t.Cleanup(func() { m.Close() })

	return m, w, out
}

func TestMux(t *testing.T) {
	t.Run("concurrent sessions never interleave within a line", func(t *testing.T) {
		m, _, out := testmux(t)
		s1, s2 := m.Session(), m.Session()

		var wg sync.WaitGroup
		for _, c := range []struct {
			s    *Session
			line string
		}{{s1, "the first session\n"}, {s2, "the second session\n"}} {
			wg.Go(func() {
				for range 200 {
					// written a few bytes at a time, as a program that does not buffer its output would.
					for chunk := range slicesChunk([]byte(c.line), 3) {
						_, err := c.s.Stdout().Write(chunk)
						require.NoError(t, err)
					}
				}
			})
		}
		wg.Wait()
		require.NoError(t, s1.Close())
		require.NoError(t, s2.Close())

		lines := strings.Split(strings.TrimSuffix(out.String(), "\r\n"), "\r\n")
		require.Len(t, lines, 400)
		for _, l := range lines {
			require.Contains(t, []string{"the first session", "the second session"}, l)
		}
	})

	t.Run("only the foreground session receives key presses", func(t *testing.T) {
		m, keys, _ := testmux(t)
		s1, s2 := m.Session(), m.Session()
		defer s1.Close()
		defer s2.Close()

		// the most recent session is the foreground.
		_, err := keys.Write([]byte("second\n"))
		require.NoError(t, err)
		line, err := bufio.NewReader(s2.Stdin()).ReadString('\n')
		require.NoError(t, err)
		require.Equal(t, "second\n", line)

		m.Foreground(s1)
		_, err = keys.Write([]byte("first\n"))
		require.NoError(t, err)
		line, err = bufio.NewReader(s1.Stdin()).ReadString('\n')
		require.NoError(t, err)
		require.Equal(t, "first\n", line)

		require.NoError(t, s2.Stdin().SetReadDeadline(time.Now().Add(100*time.Millisecond)))
		_, err = s2.Stdin().Read(make([]byte, 16))
		require.ErrorIs(t, err, os.ErrDeadlineExceeded, "a background session receives nothing")
	})

	t.Run("closing the foreground session returns the keys to the previous one", func(t *testing.T) {
		m, keys, _ := testmux(t)
		s1, s2 := m.Session(), m.Session()
		defer s1.Close()
		require.NoError(t, s2.Close())

		_, err := keys.Write([]byte("back\n"))
		require.NoError(t, err)
		line, err := bufio.NewReader(s1.Stdin()).ReadString('\n')
		require.NoError(t, err)
		require.Equal(t, "back\n", line)
	})

	t.Run("the terminal is restored when the mux is closed", func(t *testing.T) {
		_, user := testpty(t)
		original := termios(t, user)
		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer w.Close()

		// the mux closes the descriptor it is given.
		ctl, err := os.OpenFile(user.Name(), os.O_RDWR|unix.O_NOCTTY, 0)
		require.NoError(t, err)

		m, err := newmux(t.Context(), ctl, r, io.Discard)
		require.NoError(t, err)
		require.Zero(t, termios(t, user).Lflag&unix.ICANON)
		m.Session()
		require.NoError(t, m.Close())
		require.Equal(t, original, termios(t, user))
	})

	t.Run("without a terminal sessions are the process's files", func(t *testing.T) {
		var m *Mux
		s := m.Session()
		require.Equal(t, os.Stdin, s.Stdin())
		require.Equal(t, os.Stdout, s.Stdout())
		require.Equal(t, os.Stderr, s.Stderr())
		require.NoError(t, s.Close())
		require.NoError(t, m.Close())
	})
}

func slicesChunk(b []byte, n int) func(func([]byte) bool) {
	return func(yield func([]byte) bool) {
		for len(b) > 0 {
			c := b[:min(n, len(b))]
			b = b[len(c):]
			if !yield(c) {
				return
			}
		}
	}
}
