package egvm_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/stretchr/testify/require"
)

// TestExec runs commands in a machine whose root is an egiso served over FUSE. It runs as an
// unprivileged user with subordinate ids, and needs pasta and EG_TEST_ROOTFS, a tar of Alpine's
// root filesystem:
//
//	podman export $(podman create alpine:3.20) > alpine.tar
//	go test -c -o egvm.test ./internal/egvm && EG_TEST_ROOTFS=alpine.tar ./egvm.test -test.v
func TestExec(t *testing.T) {
	var (
		rootfs   = os.Getenv("EG_TEST_ROOTFS")
		console  bytes.Buffer
		archive  *egiso.Archive
		root     *egiso.SysFS
		upperdir string
		vm       *egvm.VM
		err      error
	)

	if rootfs == "" {
		t.Skip("EG_TEST_ROOTFS is not set")
	}

	// image is an egiso of the root filesystem.
	image := func(t *testing.T) *egiso.Archive {
		var (
			out bytes.Buffer
		)

		f, cause := os.Open(rootfs)
		require.NoError(t, cause)
		defer f.Close()
		st, cause := f.Stat()
		require.NoError(t, cause)

		toc, cause := egiso.Build(io.NewSectionReader(f, 0, st.Size()), &out)
		require.NoError(t, cause)
		a, cause := egiso.New(io.NewSectionReader(bytes.NewReader(out.Bytes()), 0, int64(out.Len())), toc)
		require.NoError(t, cause)
		return a
	}

	// upper is the image under a writable upper in a directory of its own.
	upper := func(t *testing.T, a *egiso.Archive) (*egiso.SysFS, string) {
		dir := t.TempDir()
		r, cause := os.OpenRoot(dir)
		require.NoError(t, cause)
		t.Cleanup(func() { r.Close() })

		tree := egiso.Tree{}.Mount(".", a, egiso.NewOwners(), egiso.NewOwners())
		return egiso.NewSysFS(egiso.NewUpper(tree, r), egiso.Default, egiso.Default), dir
	}

	archive = image(t)
	root, upperdir = upper(t, archive)
	vm, err = egvm.Boot(egvm.New(root, egvm.OptionInit("/sbin/init"), egvm.OptionMountReadOnly(t.TempDir(), "/srv/shared")), &console)
	require.NoError(t, err, console.String())
	t.Cleanup(func() { require.NoError(t, vm.Close()) })

	// run reports the command's exit code from the ExitError Exec returns for it.
	run := func(t *testing.T, ctx context.Context, stdin io.Reader, cmd string, args ...string) (int32, string, string) {
		var (
			stdout bytes.Buffer
			stderr bytes.Buffer
		)

		cause := vm.Exec(ctx, &execproxy.ExecRequest{Cmd: cmd, Arguments: args, Dir: "/", Environment: []string{"PATH=/bin:/sbin:/usr/bin"}}, egvm.ExecOptionStdin(stdin), egvm.ExecOptionStdout(&stdout), egvm.ExecOptionStderr(&stderr))
		if exit, ok := errors.AsType[egvm.ExitError](cause); ok {
			return exit.Code, stdout.String(), stderr.String()
		}
		require.NoError(t, cause, console.String())
		return 0, stdout.String(), stderr.String()
	}

	t.Run("runs the image's binaries with the egiso as their root", func(t *testing.T) {
		code, stdout, _ := run(t, t.Context(), nil, "sh", "-c", "head -1 /etc/os-release && uname -s")
		require.Equal(t, int32(0), code)
		require.Equal(t, "NAME=\"Alpine Linux\"\nLinux\n", stdout)
	})

	t.Run("runs the init as pid 1", func(t *testing.T) {
		code, stdout, _ := run(t, t.Context(), nil, "cat", "/proc/1/cmdline")
		require.Equal(t, int32(0), code)
		require.Equal(t, "/sbin/init\x00", stdout)
	})

	t.Run("mounts the machine's directories", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "sh", "-c", "touch /srv/shared/file")
		require.NotEqual(t, int32(0), code)
		require.Contains(t, stderr, "Read-only file system")
	})

	t.Run("writes go to the upper and leave the image as it is", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "sh", "-c", "echo written > /etc/written && mkdir -p /srv/data && echo data > /srv/data/file")
		require.Equal(t, int32(0), code, stderr)

		b, cause := os.ReadFile(filepath.Join(upperdir, "etc", "written"))
		require.NoError(t, cause)
		require.Equal(t, "written\n", string(b))
		b, cause = os.ReadFile(filepath.Join(upperdir, "srv", "data", "file"))
		require.NoError(t, cause)
		require.Equal(t, "data\n", string(b))
		_, cause = fs.Stat(archive, "etc/written")
		require.ErrorIs(t, cause, fs.ErrNotExist)
	})

	t.Run("returns the command's exit code", func(t *testing.T) {
		code, _, _ := run(t, t.Context(), nil, "sh", "-c", "exit 3")
		require.Equal(t, int32(3), code)
	})

	t.Run("keeps stdout and stderr apart", func(t *testing.T) {
		code, stdout, stderr := run(t, t.Context(), nil, "sh", "-c", "echo out; echo err >&2")
		require.Equal(t, int32(0), code)
		require.Equal(t, "out\n", stdout)
		require.Equal(t, "err\n", stderr)
	})

	t.Run("feeds stdin to the command until it ends", func(t *testing.T) {
		code, stdout, _ := run(t, t.Context(), strings.NewReader("piped\n"), "cat")
		require.Equal(t, int32(0), code)
		require.Equal(t, "piped\n", stdout)
	})

	t.Run("runs in the requested directory with the requested environment", func(t *testing.T) {
		var (
			stdout bytes.Buffer
		)

		cause := vm.Exec(t.Context(), &execproxy.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "pwd; echo $EGVM"}, Dir: "/etc", Environment: []string{"PATH=/bin", "EGVM=set"}}, egvm.ExecOptionStdout(&stdout))
		require.NoError(t, cause)
		require.Equal(t, "/etc\nset\n", stdout.String())
	})

	t.Run("runs with no stdio options", func(t *testing.T) {
		require.NoError(t, vm.Exec(t.Context(), &execproxy.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "echo out; echo err >&2"}, Dir: "/", Environment: []string{"PATH=/bin"}}))
	})

	t.Run("returns an ExitError for a command that fails", func(t *testing.T) {
		cause := vm.Exec(t.Context(), &execproxy.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "exit 3"}, Dir: "/", Environment: []string{"PATH=/bin"}})
		require.ErrorIs(t, cause, egvm.ExitError{Code: 3})
	})

	t.Run("exits 127 for a command the image does not hold", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "does-not-exist")
		require.Equal(t, int32(127), code)
		require.NotEmpty(t, stderr)
	})

	t.Run("runs commands concurrently", func(t *testing.T) {
		var (
			wg sync.WaitGroup
		)

		for i := range 8 {
			wg.Go(func() {
				code, stdout, _ := run(t, t.Context(), nil, "sh", "-c", fmt.Sprintf("sleep 0.2; echo %d", i))
				require.Equal(t, int32(0), code)
				require.Equal(t, fmt.Sprintf("%d\n", i), stdout)
			})
		}
		wg.Wait()
	})

	t.Run("cancelling the context kills the command", func(t *testing.T) {
		ctx, done := context.WithTimeout(t.Context(), 300*time.Millisecond)
		defer done()

		cause := vm.Exec(ctx, &execproxy.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "sleep 1; echo late > /tmp/late"}, Dir: "/", Environment: []string{"PATH=/bin"}})
		require.Error(t, cause)

		time.Sleep(1500 * time.Millisecond)
		_, cause = os.Stat(filepath.Join(upperdir, "tmp", "late"))
		require.ErrorIs(t, cause, fs.ErrNotExist)
	})
}
