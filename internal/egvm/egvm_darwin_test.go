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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/stretchr/testify/require"
)

// TestExec runs commands over ssh in a Linux machine whose root is an egiso served over NFS, booted
// by its systemd. It needs EG_TEST_ALPINE, a directory holding an uncompressed arm64 kernel (Image)
// and the kernel's modules from Alpine's modloop (modules), EG_TEST_ROOTFS, a tar of the root
// container's filesystem, whose sshd it reaches, and the test binary signed with vz.entitlements:
//
//	podman export $(podman create localhost/eg:latest) > eg.tar
//	go test -c -o egvm.test ./internal/egvm && codesign -f -s - --entitlements internal/egvm/vz.entitlements egvm.test && EG_TEST_ROOTFS=eg.tar ./egvm.test -test.v
func TestExec(t *testing.T) {
	var (
		alpine   = os.Getenv("EG_TEST_ALPINE")
		rootfs   = os.Getenv("EG_TEST_ROOTFS")
		sockets  = t.TempDir()
		owned    = t.TempDir()
		volumes  = t.TempDir()
		console  bytes.Buffer
		kernels  []string
		archive  *egiso.Archive
		root     *egiso.SysFS
		upperdir string
		users    []egiso.Account
		groups   []egiso.Account
		vm       *egvm.VM
		err      error
	)

	if alpine == "" || rootfs == "" {
		t.Skip("EG_TEST_ALPINE or EG_TEST_ROOTFS is not set")
	}
	kernels, err = filepath.Glob(filepath.Join(alpine, "modules", "*", "kernel"))
	require.NoError(t, err)
	require.Len(t, kernels, 1)

	// directory is a machine image of Alpine's kernel and an initramfs holding this vminit.
	directory := func(t *testing.T) string {
		var (
			buf bytes.Buffer
			dir = t.TempDir()
		)

		vminit := filepath.Join(t.TempDir(), "init")
		cmd := exec.Command("go", "build", "-o", vminit, "./vminit")
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0")
		b, cause := cmd.CombinedOutput()
		require.NoError(t, cause, string(b))
		b, cause = os.ReadFile(vminit)
		require.NoError(t, cause)
		require.NoError(t, egvm.MachineInitramfs(&buf, b, kernels[0]))

		require.NoError(t, os.WriteFile(filepath.Join(dir, "initramfs"), buf.Bytes(), 0600))
		require.NoError(t, os.Symlink(filepath.Join(alpine, "Image"), filepath.Join(dir, "Image")))
		return dir
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
	vm, err = egvm.Boot(egvm.New(root, egvm.OptionDirectory(directory(t)), egvm.OptionAccounts(users, groups), egvm.OptionMountReadWrite(sockets, "/srv/sockets"), egvm.OptionMountOwned(owned, "/srv/owned"), egvm.OptionMountVolume(volumes, "/srv/volume")), &console)
	if err != nil && strings.Contains(err.Error(), "entitlement") {
		t.Skipf("the test binary is not signed with vz.entitlements: %v", err)
	}
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, vm.Close()) })

	// run reports the command's exit code from the ExitError Exec returns for it.
	run := func(t *testing.T, ctx context.Context, stdin io.Reader, cmd string, args ...string) (int32, string, string) {
		var (
			stdout bytes.Buffer
			stderr bytes.Buffer
		)

		cause := vm.Exec(ctx, &execproxy.ExecRequest{Cmd: cmd, Arguments: args, Dir: "/", Environment: []string{"PATH=/bin"}}, egvm.ExecOptionStdin(stdin), egvm.ExecOptionStdout(&stdout), egvm.ExecOptionStderr(&stderr))
		if exit, ok := errors.AsType[egvm.ExitError](cause); ok {
			return exit.Code, stdout.String(), stderr.String()
		}
		require.NoError(t, cause, console.String())
		return 0, stdout.String(), stderr.String()
	}

	t.Run("runs the image's linux binaries with the egiso as their root", func(t *testing.T) {
		code, stdout, _ := run(t, t.Context(), nil, "sh", "-c", "grep ^NAME= /etc/os-release && uname -s")
		require.Equal(t, int32(0), code)
		require.Equal(t, "NAME=\"Ubuntu\"\nLinux\n", stdout)
	})

	t.Run("is given half the host's memory", func(t *testing.T) {
		host, cause := mem.VirtualMemory()
		require.NoError(t, cause)
		code, stdout, stderr := run(t, t.Context(), nil, "sh", "-c", "awk '/^MemTotal:/ { print $2 * 1024 }' /proc/meminfo")
		require.Equal(t, int32(0), code, stderr)
		total, cause := strconv.ParseUint(strings.TrimSpace(stdout), 10, 64)
		require.NoError(t, cause)
		// the kernel keeps some of the machine's memory for itself.
		require.LessOrEqual(t, total, host.Total/2)
		require.Greater(t, total, host.Total/2*9/10)
	})

	t.Run("hands pid 1 to the root's systemd", func(t *testing.T) {
		code, stdout, _ := run(t, t.Context(), nil, "cat", "/proc/1/comm")
		require.Equal(t, int32(0), code)
		require.Equal(t, "systemd\n", stdout)
	})

	t.Run("mounts a volume of its own, formatted ext4", func(t *testing.T) {
		code, stdout, stderr := run(t, t.Context(), nil, "findmnt", "-n", "-o", "FSTYPE", "/srv/volume")
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "ext4\n", stdout)

		st, cause := os.Stat(filepath.Join(volumes, "volume.img"))
		require.NoError(t, cause)
		require.Equal(t, int64(128<<30), st.Size())
	})

	t.Run("refuses to attach a volume another machine holds", func(t *testing.T) {
		_, cause := egvm.Boot(egvm.New(root, egvm.OptionDirectory(directory(t)), egvm.OptionMountVolume(volumes, "/srv/volume")), io.Discard)
		require.ErrorContains(t, cause, "attached to another machine")
	})

	t.Run("gives the host user what any of the machine's users writes in an owned directory", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "sh", "-c", "echo root > /srv/owned/root && sudo -u egd sh -c 'echo egd > /srv/owned/egd'")
		require.Equal(t, int32(0), code, stderr)

		for _, name := range []string{"root", "egd"} {
			st, cause := os.Lstat(filepath.Join(owned, name))
			require.NoError(t, cause)
			require.Equal(t, uint32(os.Getuid()), st.Sys().(*syscall.Stat_t).Uid, name)
		}
	})

	t.Run("a volume holds what containers' storage writes", func(t *testing.T) {
		code, stdout, stderr := run(t, t.Context(), nil, "sh", "-c", strings.Join([]string{
			"cd /srv/volume",
			"mkdir -p lower upper work merged",
			"chown 100000:100000 lower",
			"mknod null c 1 3",
			"mount -t overlay overlay -o lowerdir=lower,upperdir=upper,workdir=work merged",
			"stat -c %u lower",
			"umount merged",
		}, " && "))
		require.Equal(t, int32(0), code, stderr)
		require.Equal(t, "100000\n", stdout)
	})

	t.Run("forwards a host socket into the machine", func(t *testing.T) {
		// macOS limits a unix socket's path to 104 bytes, which a test's temporary directory exceeds.
		dir, cause := os.MkdirTemp("/tmp", "egvm")
		require.NoError(t, cause)
		t.Cleanup(func() { os.RemoveAll(dir) })
		host := filepath.Join(dir, "host.sock")
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
		require.Error(t, vm.Forward(t.Context(), "/tmp/egvm.missing.sock", "/srv/missing/forwarded.sock"))
	})

	t.Run("quotes arguments the shell would expand", func(t *testing.T) {
		code, stdout, _ := run(t, t.Context(), nil, "printf", "%s|", "a b", "$HOME", "'q'", "*")
		require.Equal(t, int32(0), code)
		require.Equal(t, "a b|$HOME|'q'|*|", stdout)
	})

	t.Run("writes go to the upper and leave the image as it is", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "sh", "-c", "echo written > /etc/written && mkdir -p /srv && echo data > /srv/data")
		require.Equal(t, int32(0), code, stderr)

		b, cause := os.ReadFile(filepath.Join(upperdir, "etc", "written"))
		require.NoError(t, cause)
		require.Equal(t, "written\n", string(b))
		b, cause = os.ReadFile(filepath.Join(upperdir, "srv", "data"))
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
		require.ErrorIs(t, cause, context.DeadlineExceeded)

		time.Sleep(1500 * time.Millisecond)
		_, cause = os.Stat(filepath.Join(upperdir, "tmp", "late"))
		require.ErrorIs(t, cause, fs.ErrNotExist)
	})
}
