//go:build linux

package cmdsecret_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdsecret"
	cmdtestx "github.com/egdaemon/eg/cmd/cmdtestx/v2"
	"github.com/egdaemon/eg/internal/ttyx"
	"github.com/egdaemon/eg/secrets"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestSecretCreateInput(t *testing.T) {
	str := func(s string) *string { return &s }

	tests := []struct {
		name       string
		stdin      *string // nil is a pipe with nothing in it.
		tty        bool    // the default terminal is a pty.
		pty        bool    // a pty exists, {pty} in args and envtty is its path.
		envtty     string  // EG_TTY.
		editor     string  // shell run by the editor after it captured the file it was given.
		args       []string
		wantEditor bool
		wantSeen   string
		wantStored *string // nil when no secret is expected to exist.
		wantErr    string
	}{
		{
			name:       "stdin and tty opens the editor with stdin and writes the edited result",
			stdin:      str("seed\n"),
			tty:        true,
			editor:     `printf 'more\n' >> "$1"`,
			wantEditor: true,
			wantSeen:   "seed\n",
			wantStored: str("seed\nmore\n"),
		},
		{
			name:       "stdin without a tty writes stdin and does not open the editor",
			stdin:      str("piped"),
			editor:     `printf 'unexpected' > "$1"`,
			wantStored: str("piped"),
		},
		{
			name:       "no stdin with a tty opens the editor empty and writes the result",
			tty:        true,
			editor:     `printf 'typed' > "$1"`,
			wantEditor: true,
			wantSeen:   "",
			wantStored: str("typed"),
		},
		{
			name:    "no stdin and no tty is an error",
			editor:  `printf 'unexpected' > "$1"`,
			wantErr: "allow-empty",
		},
		{
			name:       "no stdin and no tty writes an empty secret when allowed",
			editor:     `printf 'unexpected' > "$1"`,
			args:       []string{"--allow-empty"},
			wantStored: str(""),
		},
		{
			name:       "an empty editor is an error",
			tty:        true,
			editor:     `: > "$1"`,
			wantEditor: true,
			wantErr:    "allow-empty",
		},
		{
			name:       "an empty editor writes an empty secret when allowed",
			tty:        true,
			editor:     `: > "$1"`,
			args:       []string{"--allow-empty"},
			wantEditor: true,
			wantStored: str(""),
		},
		{
			name:       "tty flag to a missing device means no editor even when a terminal is available",
			stdin:      str("piped"),
			tty:        true,
			editor:     `printf 'unexpected' > "$1"`,
			args:       []string{"--tty=/nonexistent/tty"},
			wantStored: str("piped"),
		},
		{
			name:       "tty flag selects the device the editor opens on",
			stdin:      str("seed\n"),
			pty:        true,
			editor:     `printf 'more\n' >> "$1"`,
			args:       []string{"--tty={pty}"},
			wantEditor: true,
			wantSeen:   "seed\n",
			wantStored: str("seed\nmore\n"),
		},
		{
			name:       "environment selects the device the editor opens on",
			stdin:      str("seed\n"),
			pty:        true,
			envtty:     "{pty}",
			editor:     `printf 'more\n' >> "$1"`,
			wantEditor: true,
			wantSeen:   "seed\n",
			wantStored: str("seed\nmore\n"),
		},
		{
			name:       "environment naming a missing device means no editor",
			stdin:      str("piped"),
			tty:        true,
			envtty:     "/nonexistent/tty",
			editor:     `printf 'unexpected' > "$1"`,
			wantStored: str("piped"),
		},
		{
			name:       "tty flag beats the environment",
			stdin:      str("piped"),
			pty:        true,
			envtty:     "{pty}",
			editor:     `printf 'unexpected' > "$1"`,
			args:       []string{"--tty=/nonexistent/tty"},
			wantStored: str("piped"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			path := filepath.Join(tmp, "create.chacha")
			capture := filepath.Join(tmp, "capture.txt")
			uri := "chachasm://pass@" + path

			editor := filepath.Join(tmp, "editor.sh")
			require.NoError(t, os.WriteFile(editor, []byte(fmt.Sprintf("#!/bin/sh\ncat \"$1\" > %q\n%s\n", capture, tc.editor)), 0o700))
			t.Setenv("EDITOR", editor)

			r, w, err := os.Pipe()
			require.NoError(t, err)
			defer r.Close()
			if tc.stdin != nil {
				_, err = w.Write([]byte(*tc.stdin))
				require.NoError(t, err)
			}
			require.NoError(t, w.Close())

			ttypath := "/nonexistent/tty"
			ptypath := "/nonexistent/pty"
			if tc.tty || tc.pty {
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
				ptypath = fmt.Sprintf("/dev/pts/%d", n)
				if tc.tty {
					ttypath = ptypath
				}
			}
			defer ttyx.Override(func() *os.File { return r }, ttypath)()
			defer ttyx.SetPath("")

			if tc.envtty != "" {
				t.Setenv(eg.EnvTTY, strings.ReplaceAll(tc.envtty, "{pty}", ptypath))
			}

			args := make([]string, 0, len(tc.args))
			for _, a := range tc.args {
				args = append(args, strings.ReplaceAll(a, "{pty}", ptypath))
			}

			var cli struct {
				Secret cmdsecret.SecretCmd `cmd:""`
			}
			err = cmdtestx.New(t, &cli).Run(append([]string{"secret", "create", uri}, args...)...)

			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			seen, err := os.ReadFile(capture)
			if tc.wantEditor {
				require.NoError(t, err, "the editor was expected to run")
				require.Equal(t, tc.wantSeen, string(seen))
			} else {
				require.ErrorIs(t, err, os.ErrNotExist, "the editor was not expected to run")
			}

			if tc.wantStored == nil {
				require.NoFileExists(t, path)
				return
			}

			got, err := io.ReadAll(secrets.Read(t.Context(), uri))
			require.NoError(t, err)
			require.Equal(t, *tc.wantStored, string(got))
		})
	}
}
