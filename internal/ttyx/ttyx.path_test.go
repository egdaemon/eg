//go:build linux

package ttyx

import (
	"os"
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/stretchr/testify/require"
)

func TestAvailable(t *testing.T) {
	tests := []struct {
		name     string
		terminal bool   // stdin is the pty child, otherwise a pipe.
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
			_, child := testpty(t)
			ptypath := child.Name()

			r, w, err := os.Pipe()
			require.NoError(t, err)
			defer r.Close()
			defer w.Close()

			in := r
			if tc.terminal {
				in = child
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
