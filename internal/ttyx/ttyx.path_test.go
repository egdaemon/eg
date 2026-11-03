//go:build linux

package ttyx

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestAvailable(t *testing.T) {
	tests := []struct {
		name     string
		terminal bool   // stdin is the pty slave, otherwise a pipe.
		def      string // ttypath, {pty} is the pty.
		env      string // EG_TTY
		flag     string // SetPath
		want     bool
	}{
		{name: "the default device", def: "{pty}", want: true},
		{name: "no default device", def: "/nonexistent/tty", want: false},
		{name: "stdin is a terminal", terminal: true, def: "/nonexistent/tty", want: true},
		{name: "environment device is used instead of the default", def: "/nonexistent/tty", env: "{pty}", want: true},
		{name: "environment naming a missing device means no terminal", def: "{pty}", env: "/nonexistent/tty", want: false},
		{name: "environment naming a missing device beats a terminal stdin", terminal: true, def: "{pty}", env: "/nonexistent/tty", want: false},
		{name: "flag device is used instead of the default", def: "/nonexistent/tty", flag: "{pty}", want: true},
		{name: "flag naming a missing device beats a terminal stdin", terminal: true, def: "{pty}", flag: "/nonexistent/tty", want: false},
		{name: "flag beats the environment", def: "/nonexistent/tty", env: "/nonexistent/tty", flag: "{pty}", want: true},
		{name: "flag naming a missing device beats the environment", def: "{pty}", env: "{pty}", flag: "/nonexistent/tty", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
			if err != nil {
				t.Skip("no pseudo terminal available", err)
			}
			defer master.Close()

			require.NoError(t, unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0))
			n, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
			require.NoError(t, err)
			ptypath := fmt.Sprintf("/dev/pts/%d", n)
			slave, err := os.OpenFile(ptypath, os.O_RDWR|unix.O_NOCTTY, 0)
			require.NoError(t, err)
			defer slave.Close()

			r, w, err := os.Pipe()
			require.NoError(t, err)
			defer r.Close()
			defer w.Close()

			in := r
			if tc.terminal {
				in = slave
			}
			defer Override(func() *os.File { return in }, strings.ReplaceAll(tc.def, "{pty}", ptypath))()
			defer SetPath("")

			if tc.env != "" {
				t.Setenv(eg.EnvTTY, strings.ReplaceAll(tc.env, "{pty}", ptypath))
			}
			SetPath(strings.ReplaceAll(tc.flag, "{pty}", ptypath))

			require.Equal(t, tc.want, Available())
		})
	}
}
