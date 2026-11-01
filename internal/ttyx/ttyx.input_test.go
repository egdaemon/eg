//go:build linux

package ttyx

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/mattn/go-isatty"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestInput(t *testing.T) {
	t.Run("stdin is used when it is a terminal and is not closed with the result", func(t *testing.T) {
		master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
		if err != nil {
			t.Skip("no pseudo terminal available", err)
		}
		defer master.Close()

		require.NoError(t, unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0))
		n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
		require.NoError(t, err)
		slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
		require.NoError(t, err)
		defer slave.Close()
		stdin = func() *os.File { return slave }
		ttypath = "/nonexistent/tty"

		in := Input(t.Context())
		f, ok := in.(*os.File)
		require.True(t, ok, "an *os.File is what lets a program run by os/exec see the terminal")
		require.True(t, isatty.IsTerminal(f.Fd()))

		_, err = master.Write([]byte("typed\n"))
		require.NoError(t, err)
		buf := make([]byte, 6)
		_, err = io.ReadFull(in, buf)
		require.NoError(t, err)
		require.Equal(t, "typed\n", string(buf))

		require.NoError(t, in.Close())
		_, err = slave.Stat()
		require.NoError(t, err)
	})

	t.Run("the controlling terminal is used when stdin is piped", func(t *testing.T) {
		master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
		if err != nil {
			t.Skip("no pseudo terminal available", err)
		}
		defer master.Close()

		require.NoError(t, unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0))
		n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
		require.NoError(t, err)
		slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
		require.NoError(t, err)
		defer slave.Close()

		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer r.Close()
		defer w.Close()
		stdin = func() *os.File { return r }
		ttypath = fmt.Sprintf("/dev/pts/%d", n)

		in := Input(t.Context())
		defer in.Close()

		_, err = master.Write([]byte("typed\n"))
		require.NoError(t, err)
		buf := make([]byte, 6)
		_, err = io.ReadFull(in, buf)
		require.NoError(t, err)
		require.Equal(t, "typed\n", string(buf))
	})

	t.Run("stdin is used when there is no terminal and is not closed with the result", func(t *testing.T) {
		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer r.Close()
		defer w.Close()
		stdin = func() *os.File { return r }
		ttypath = "/nonexistent/tty"

		_, err = w.Write([]byte("scripted"))
		require.NoError(t, err)

		in := Input(t.Context())
		buf := make([]byte, 8)
		_, err = io.ReadFull(in, buf)
		require.NoError(t, err)
		require.Equal(t, "scripted", string(buf))

		require.NoError(t, in.Close())
		_, err = r.Stat()
		require.NoError(t, err)
	})
}
