package shell

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInvoke(t *testing.T) {
	script := func(t *testing.T, dir string, name string, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
	}

	expect := func(t *testing.T, inv invocation, ok bool, name string, src string, params ...string) {
		require.True(t, ok)
		b, err := io.ReadAll(inv.src)
		require.NoError(t, err)
		require.Equal(t, name, inv.name)
		require.Equal(t, src, string(b))
		require.Equal(t, params, inv.params)
	}

	t.Run("other commands go to the module", func(t *testing.T) {
		_, ok := invoke(t.TempDir(), nil, []string{"go", "build", "./..."})
		require.False(t, ok)
	})

	t.Run("-c names the shell as $0", func(t *testing.T) {
		inv, ok := invoke(t.TempDir(), nil, []string{"bash", "-c", "echo hi"})
		expect(t, inv, ok, "bash", "echo hi", "--")
	})

	t.Run("-c takes $0 and the positional parameters after the string", func(t *testing.T) {
		inv, ok := invoke(t.TempDir(), nil, []string{"/bin/sh", "-c", "echo $1", "name", "a", "-b"})
		expect(t, inv, ok, "name", "echo $1", "--", "a", "-b")
	})

	t.Run("set options become params", func(t *testing.T) {
		inv, ok := invoke(t.TempDir(), nil, []string{"bash", "-eu", "+x", "-o", "pipefail", "-c", "true"})
		expect(t, inv, ok, "bash", "true", "-e", "-u", "+x", "-o", "pipefail", "--")
	})

	t.Run("-c combines with other options", func(t *testing.T) {
		inv, ok := invoke(t.TempDir(), nil, []string{"sh", "-ec", "true"})
		expect(t, inv, ok, "sh", "true", "-e", "--")
	})

	t.Run("a script file relative to the directory", func(t *testing.T) {
		dir := t.TempDir()
		script(t, dir, "build.sh", "echo build")
		inv, ok := invoke(dir, nil, []string{"bash", "-x", "build.sh", "a"})
		expect(t, inv, ok, "build.sh", "echo build", "-x", "--", "a")
	})

	t.Run("-- ends the options", func(t *testing.T) {
		dir := t.TempDir()
		script(t, dir, "-e", "echo dash")
		inv, ok := invoke(dir, nil, []string{"sh", "--", "-e"})
		expect(t, inv, ok, "-e", "echo dash", "--")
	})

	t.Run("no script reads stdin", func(t *testing.T) {
		inv, ok := invoke(t.TempDir(), strings.NewReader("echo piped"), []string{"sh"})
		expect(t, inv, ok, "sh", "echo piped", "--")
		require.Nil(t, inv.stdin)
	})

	t.Run("no script and no stdin is an empty script", func(t *testing.T) {
		inv, ok := invoke(t.TempDir(), nil, []string{"sh", "-e"})
		expect(t, inv, ok, "sh", "", "-e", "--")
	})

	t.Run("-c and script files keep stdin for their commands", func(t *testing.T) {
		stdin := strings.NewReader("input")
		inv, ok := invoke(t.TempDir(), stdin, []string{"sh", "-c", "cat"})
		require.True(t, ok)
		require.Equal(t, stdin, inv.stdin)
	})

	t.Run("unsupported options go to the module", func(t *testing.T) {
		for _, args := range [][]string{
			{"bash", "--norc", "-c", "true"},
			{"bash", "-l", "-c", "true"},
			{"bash", "+c", "true"},
			{"bash", "-o"},
			{"bash", "-c"},
		} {
			_, ok := invoke(t.TempDir(), nil, args)
			require.False(t, ok, args)
		}
	})

	t.Run("a script the guest cannot read goes to the module", func(t *testing.T) {
		_, ok := invoke(t.TempDir(), nil, []string{"sh", "missing.sh"})
		require.False(t, ok)
	})

	t.Run("a shebang naming sh, with its argument", func(t *testing.T) {
		dir := t.TempDir()
		script(t, dir, "run.sh", "#!/bin/sh -e\necho run\n")
		inv, ok := invoke(dir, nil, []string{"./run.sh", "a"})
		expect(t, inv, ok, "./run.sh", "#!/bin/sh -e\necho run\n", "-e", "--", "a")
	})

	t.Run("a shebang naming bash through env", func(t *testing.T) {
		dir := t.TempDir()
		script(t, dir, "run", "#!/usr/bin/env bash")
		inv, ok := invoke(dir, nil, []string{filepath.Join(dir, "run")})
		expect(t, inv, ok, filepath.Join(dir, "run"), "#!/usr/bin/env bash", "--")
	})

	t.Run("a shebang naming another interpreter goes to the module", func(t *testing.T) {
		dir := t.TempDir()
		script(t, dir, "run.py", "#!/usr/bin/env python3\nprint('hi')\n")
		_, ok := invoke(dir, nil, []string{"./run.py"})
		require.False(t, ok)
	})

	t.Run("a file without a shebang goes to the module", func(t *testing.T) {
		dir := t.TempDir()
		script(t, dir, "bin", "\x7fELF")
		_, ok := invoke(dir, nil, []string{"./bin"})
		require.False(t, ok)
	})

	t.Run("a path the guest cannot read goes to the module", func(t *testing.T) {
		_, ok := invoke(t.TempDir(), nil, []string{"./missing"})
		require.False(t, ok)
	})
}
