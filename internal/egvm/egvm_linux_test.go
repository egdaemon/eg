package egvm_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/stretchr/testify/require"
)

// TestExec runs commands over ssh in a machine whose root is an egiso served over FUSE, booted by
// its systemd. It runs as an unprivileged user with subordinate ids and a systemd of its own, and
// needs pasta and EG_TEST_ROOTFS, a tar of the root container's filesystem, whose sshd it reaches:
//
//	podman export $(podman create localhost/eg:latest) > eg.tar
//	go test -c -o egvm.test ./internal/egvm && EG_TEST_ROOTFS=eg.tar ./egvm.test -test.v
func TestExec(t *testing.T) {
	var (
		rootfs   = os.Getenv("EG_TEST_ROOTFS")
		sockets  = t.TempDir()
		binary   = filepath.Join(t.TempDir(), "binary")
		owned    = t.TempDir()
		volume   = filepath.Join(t.TempDir(), "volume")
		console  bytes.Buffer
		archive  *egiso.Archive
		root     *egiso.SysFS
		upperdir string
		users    []egiso.Account
		groups   []egiso.Account
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

	// upper is the image under a writable upper in a directory of its own, keeping the image's own
	// owners, as runners.Machine roots a machine, so sshd finds its files owned by root.
	upper := func(t *testing.T, a *egiso.Archive) (*egiso.SysFS, string) {
		dir := t.TempDir()
		r, cause := os.OpenRoot(dir)
		require.NoError(t, cause)
		t.Cleanup(func() { r.Close() })

		tree := egiso.Tree{}.Mount(".", a, egiso.Identity(users), egiso.Identity(groups))
		return egiso.NewSysFS(egiso.NewUpper(tree, r), egiso.Default, egiso.Default), dir
	}

	archive = image(t)
	users, groups, err = archive.Accounts()
	require.NoError(t, err)
	root, upperdir = upper(t, archive)
	require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\necho bound\n"), 0755))
	vm, err = egvm.Boot(egvm.New(root, egvm.OptionAccounts(users, groups), egvm.OptionCPUs(2), egvm.OptionMemory(1<<30), egvm.OptionMountReadOnly(t.TempDir(), "/srv/shared"), egvm.OptionMountReadWrite(sockets, "/srv/sockets"), egvm.OptionMountReadOnly(binary, "/srv/sockets/bin/binary"), egvm.OptionMountOwned(owned, "/srv/owned"), egvm.OptionMountVolume(volume, "/srv/volume")), &console)
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
		code, stdout, _ := run(t, t.Context(), nil, "sh", "-c", "grep ^NAME= /etc/os-release && uname -s")
		require.Equal(t, int32(0), code)
		require.Equal(t, "NAME=\"Ubuntu\"\nLinux\n", stdout)
	})

	t.Run("runs the root's systemd as pid 1", func(t *testing.T) {
		code, stdout, _ := run(t, t.Context(), nil, "cat", "/proc/1/comm")
		require.Equal(t, int32(0), code)
		require.Equal(t, "systemd\n", stdout)
	})

	t.Run("names the machine as its systemd's container manager", func(t *testing.T) {
		code, stdout, stderr := run(t, t.Context(), nil, "cat", "/run/systemd/container")
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, egvm.Container+"\n", stdout)
	})

	t.Run("lets its systemd hand its controllers down, as podman's containers need", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "sh", "-c", "grep -qw pids /sys/fs/cgroup/cgroup.subtree_control || echo +pids > /sys/fs/cgroup/cgroup.subtree_control")
		require.Equal(t, int32(0), code, stderr)
	})

	t.Run("limits the machine to its cpus and memory", func(t *testing.T) {
		code, stdout, stderr := run(t, t.Context(), nil, "cat", "/sys/fs/cgroup/cpu.max", "/sys/fs/cgroup/memory.max")
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "200000 100000\n1073741824\n", stdout)
	})

	t.Run("links files, as dpkg backs up its status", func(t *testing.T) {
		code, stdout, stderr := run(t, t.Context(), nil, "sh", "-c", "echo linked > /etc/status && ln /etc/status /etc/status-old && cat /etc/status-old && stat -c %h /etc/status-old")
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "linked\n2\n", stdout)
	})

	t.Run("drops what it sends to private addresses", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "curl", "-sS", "--max-time", "2", "http://169.254.169.254/")
		require.Equal(t, int32(28), code, stderr)
	})

	t.Run("keeps its firewall out of root's reach", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "nft", "flush", "ruleset")
		require.NotEqual(t, int32(0), code)
		require.Contains(t, stderr, "Operation not permitted")
	})

	t.Run("quotes arguments the shell would expand", func(t *testing.T) {
		code, stdout, _ := run(t, t.Context(), nil, "printf", "%s|", "a b", "$HOME", "'q'", "*")
		require.Equal(t, int32(0), code)
		require.Equal(t, "a b|$HOME|'q'|*|", stdout)
	})

	t.Run("mounts a volume in the directory it creates for it", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "sh", "-c", "echo kept > /srv/volume/kept && chown 1000:1000 /srv/volume/kept")
		require.Equal(t, int32(0), code, stderr)
		b, cause := os.ReadFile(filepath.Join(volume, "kept"))
		require.NoError(t, cause)
		require.Equal(t, "kept\n", string(b))
	})

	t.Run("mounts a host file read only within a directory it shares, as podman mounts egbin", func(t *testing.T) {
		code, stdout, stderr := run(t, t.Context(), nil, "sh", "-c", "/srv/sockets/bin/binary && ! touch /srv/sockets/bin/binary 2>/dev/null")
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "bound\n", stdout)

		placeholder, cause := os.Stat(filepath.Join(sockets, "bin", "binary"))
		require.NoError(t, cause)
		require.Zero(t, placeholder.Size())
	})

	t.Run("forwards a host socket into the machine", func(t *testing.T) {
		host := filepath.Join(t.TempDir(), "host.sock")
		l, cause := net.Listen("unix", host)
		require.NoError(t, cause)
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "forwarded") })}
		go srv.Serve(l)
		t.Cleanup(func() { srv.Close() })

		require.NoError(t, vm.Forward(t.Context(), host, "/srv/sockets/forwarded.sock"))
		code, stdout, stderr := run(t, t.Context(), nil, "curl", "-s", "--unix-socket", "/srv/sockets/forwarded.sock", "http://machine/")
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "forwarded", stdout)
	})

	t.Run("refuses to forward to a path the machine can not listen on", func(t *testing.T) {
		require.Error(t, vm.Forward(t.Context(), filepath.Join(t.TempDir(), "host.sock"), "/srv/missing/forwarded.sock"))
	})

	t.Run("mounts the machine's directories", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "sh", "-c", "touch /srv/shared/file")
		require.NotEqual(t, int32(0), code)
		require.Contains(t, stderr, "Read-only file system")
	})

	t.Run("gives the host user what any of the machine's users writes in an owned directory", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "sh", "-c", "mkdir /srv/owned/root && echo root > /srv/owned/root/file && sudo -u egd sh -c 'mkdir /srv/owned/egd && echo egd > /srv/owned/egd/file'")
		require.Equal(t, int32(0), code, stderr)

		for _, name := range []string{"root", "root/file", "egd", "egd/file"} {
			st, cause := os.Lstat(filepath.Join(owned, name))
			require.NoError(t, cause)
			require.Equal(t, uint32(os.Getuid()), st.Sys().(*syscall.Stat_t).Uid, name)
		}
		require.NoError(t, os.RemoveAll(filepath.Join(owned, "root")))
		require.NoError(t, os.RemoveAll(filepath.Join(owned, "egd")))
	})

	t.Run("shows each user an owned directory's files as its own", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(owned, "host"), []byte("host\n"), 0644))
		code, stdout, stderr := run(t, t.Context(), nil, "sh", "-c", "stat -c %u:%g /srv/owned/host && sudo -u egd stat -c %U:%G /srv/owned/host && setpriv --reuid 4242 --regid 4242 --clear-groups stat -c %u:%g /srv/owned/host")
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "0:0\n"+egiso.Default+":"+egiso.Default+"\n4242:4242\n", stdout)
	})

	t.Run("lets a user the image does not name use what other users made private in an owned directory", func(t *testing.T) {
		code, stdout, stderr := run(t, t.Context(), nil, "sh", "-c", "mkdir -m 0700 /srv/owned/private && setpriv --reuid 4242 --regid 4242 --clear-groups sh -c 'echo written > /srv/owned/private/file && cat /srv/owned/private/file'")
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "written\n", stdout)

		st, cause := os.Lstat(filepath.Join(owned, "private", "file"))
		require.NoError(t, cause)
		require.Equal(t, uint32(os.Getuid()), st.Sys().(*syscall.Stat_t).Uid)
		require.NoError(t, os.RemoveAll(filepath.Join(owned, "private")))
	})

	t.Run("lets root change what the host user made read only in an owned directory, as root may", func(t *testing.T) {
		require.NoError(t, os.MkdirAll(filepath.Join(owned, "readonly", "module"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(owned, "readonly", "module", "go.mod"), nil, 0444))
		require.NoError(t, os.Chmod(filepath.Join(owned, "readonly", "module"), 0555))
		require.NoError(t, os.Chmod(filepath.Join(owned, "readonly"), 0555))

		code, _, _ := run(t, t.Context(), nil, "sudo", "-u", "egd", "rm", "-rf", "/srv/owned/readonly/module")
		require.NotEqual(t, int32(0), code)

		code, _, stderr := run(t, t.Context(), nil, "sh", "-c", "rm -rf /srv/owned/readonly/module && touch /srv/owned/readonly/added")
		require.Equal(t, int32(0), code, stderr)
		require.NoFileExists(t, filepath.Join(owned, "readonly", "module", "go.mod"))
		require.FileExists(t, filepath.Join(owned, "readonly", "added"))
		st, cause := os.Stat(filepath.Join(owned, "readonly"))
		require.NoError(t, cause)
		require.Equal(t, fs.FileMode(0555), st.Mode().Perm())

		require.NoError(t, os.Chmod(filepath.Join(owned, "readonly"), 0755))
		require.NoError(t, os.RemoveAll(filepath.Join(owned, "readonly")))
	})

	t.Run("lets the machine's processes connect to a socket bound in an owned directory", func(t *testing.T) {
		code, stdout, stderr := run(t, t.Context(), nil, "python3", "-c", strings.Join([]string{
			"import socket",
			"s = socket.socket(socket.AF_UNIX); s.bind('/srv/owned/bound.socket'); s.listen()",
			"c = socket.socket(socket.AF_UNIX); c.connect('/srv/owned/bound.socket'); c.sendall(b'bound')",
			"print(s.accept()[0].recv(5).decode())",
		}, "; "))
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "bound\n", stdout)

		st, cause := os.Lstat(filepath.Join(owned, "bound.socket"))
		require.NoError(t, cause)
		require.Equal(t, fs.ModeSocket, st.Mode().Type())
		require.Equal(t, uint32(os.Getuid()), st.Sys().(*syscall.Stat_t).Uid)
		require.NoError(t, os.Remove(filepath.Join(owned, "bound.socket")))
	})

	t.Run("lets a user change the owner of a file in an owned directory, which stays the host user's", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(owned, "chowned"), nil, 0644))
		code, stdout, stderr := run(t, t.Context(), nil, "sh", "-c", "chown egd:egd /srv/owned/chowned && stat -c %u:%g /srv/owned/chowned")
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "0:0\n", stdout)

		st, cause := os.Lstat(filepath.Join(owned, "chowned"))
		require.NoError(t, cause)
		require.Equal(t, uint32(os.Getuid()), st.Sys().(*syscall.Stat_t).Uid)
	})

	t.Run("reports the host filesystem holding an owned directory", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "df", "/srv/owned")
		require.Equal(t, int32(0), code, stderr)
	})

	t.Run("sees what the host writes in an owned directory after it looked", func(t *testing.T) {
		code, _, _ := run(t, t.Context(), nil, "cat", "/srv/owned/later")
		require.NotEqual(t, int32(0), code)
		require.NoError(t, os.WriteFile(filepath.Join(owned, "later"), []byte("later\n"), 0644))
		code, stdout, stderr := run(t, t.Context(), nil, "cat", "/srv/owned/later")
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "later\n", stdout)
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
