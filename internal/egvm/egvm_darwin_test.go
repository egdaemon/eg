package egvm_test

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
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

// TestExec runs commands in a Linux machine whose root is an egiso served over NFS. It needs
// EG_TEST_ALPINE, a directory holding an uncompressed arm64 kernel (Image), Alpine's initramfs
// unpacked (initrd) and the kernel's modules from its modloop (modules), and the test binary signed
// with vz.entitlements:
//
//	go test -c -o egvm.test ./internal/egvm && codesign -f -s - --entitlements internal/egvm/vz.entitlements egvm.test && ./egvm.test -test.v
func TestExec(t *testing.T) {
	alpine := os.Getenv("EG_TEST_ALPINE")
	if alpine == "" {
		t.Skip("EG_TEST_ALPINE is not set")
	}
	initrd := filepath.Join(alpine, "initrd")
	kernels, err := filepath.Glob(filepath.Join(alpine, "modules", "*", "kernel"))
	require.NoError(t, err)
	require.Len(t, kernels, 1)

	// initramfs holds vminit and the modules virtio-fs and vsock need, loaded in name order.
	initramfs := func(t *testing.T) string {
		var (
			buf bytes.Buffer
		)

		vminit := filepath.Join(t.TempDir(), "init")
		cmd := exec.Command("go", "build", "-o", vminit, "./vminit")
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0")
		b, err := cmd.CombinedOutput()
		require.NoError(t, err, string(b))

		read := func(name string) []byte {
			b, err := os.ReadFile(name)
			require.NoError(t, err)
			return b
		}
		module := func(name string) []byte {
			return read(filepath.Join(kernels[0], name))
		}

		require.NoError(t, egvm.Initramfs(&buf,
			egvm.File{Name: "proc", Mode: fs.ModeDir | 0755},
			egvm.File{Name: "sys", Mode: fs.ModeDir | 0755},
			egvm.File{Name: "dev", Mode: fs.ModeDir | 0755},
			egvm.File{Name: "root", Mode: fs.ModeDir | 0755},
			egvm.File{Name: "modules", Mode: fs.ModeDir | 0755},
			egvm.File{Name: "modules/1.fuse.ko", Mode: 0644, Data: module("fs/fuse/fuse.ko")},
			egvm.File{Name: "modules/2.virtiofs.ko", Mode: 0644, Data: module("fs/fuse/virtiofs.ko")},
			egvm.File{Name: "modules/3.vsock.ko", Mode: 0644, Data: module("net/vmw_vsock/vsock.ko")},
			egvm.File{Name: "modules/4.vmw_vsock_virtio_transport_common.ko", Mode: 0644, Data: module("net/vmw_vsock/vmw_vsock_virtio_transport_common.ko")},
			egvm.File{Name: "modules/5.vmw_vsock_virtio_transport.ko", Mode: 0644, Data: module("net/vmw_vsock/vmw_vsock_virtio_transport.ko")},
			egvm.File{Name: "init", Mode: 0755, Data: read(vminit)},
		))

		path := filepath.Join(t.TempDir(), "initramfs")
		require.NoError(t, os.WriteFile(path, buf.Bytes(), 0600))
		return path
	}

	// image is an egiso of Alpine's busybox and musl.
	image := func(t *testing.T) *egiso.Archive {
		var (
			buf bytes.Buffer
			out bytes.Buffer
		)

		tw := tar.NewWriter(&buf)
		write := func(h *tar.Header, data []byte) {
			h.Size = int64(len(data))
			h.ModTime = time.Unix(1700000000, 0)
			require.NoError(t, tw.WriteHeader(h))
			_, err := tw.Write(data)
			require.NoError(t, err)
		}
		host := func(name string) []byte {
			b, err := os.ReadFile(filepath.Join(initrd, name))
			require.NoError(t, err)
			return b
		}

		for _, dir := range []string{"bin/", "lib/", "etc/", "tmp/"} {
			write(&tar.Header{Typeflag: tar.TypeDir, Name: dir, Mode: 0755}, nil)
		}
		write(&tar.Header{Typeflag: tar.TypeReg, Name: "bin/busybox", Mode: 0755}, host("bin/busybox"))
		for _, name := range []string{"sh", "cat", "uname", "mkdir", "sleep", "pwd"} {
			write(&tar.Header{Typeflag: tar.TypeSymlink, Name: "bin/" + name, Linkname: "/bin/busybox", Mode: 0777}, nil)
		}
		write(&tar.Header{Typeflag: tar.TypeReg, Name: "lib/ld-musl-aarch64.so.1", Mode: 0755}, host("lib/ld-musl-aarch64.so.1"))
		write(&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644}, []byte("egvm\n"))
		require.NoError(t, tw.Close())

		toc, err := egiso.Build(io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len())), &out)
		require.NoError(t, err)
		archive, err := egiso.New(io.NewSectionReader(bytes.NewReader(out.Bytes()), 0, int64(out.Len())), toc)
		require.NoError(t, err)
		return archive
	}

	// mounted serves an upper over the image over NFS and mounts it on the host, so the machine can
	// share it.
	mounted := func(t *testing.T, archive *egiso.Archive) (string, string) {
		upperdir := t.TempDir()
		dir, err := os.OpenRoot(upperdir)
		require.NoError(t, err)
		t.Cleanup(func() { dir.Close() })

		tree := egiso.Tree{}.Mount(".", archive, egiso.NewOwners(), egiso.NewOwners())
		fsys := egiso.NewSysFS(egiso.NewUpper(tree, dir), egiso.Default, egiso.Default)

		l, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		go egiso.ServeNFS(l, fsys)
		t.Cleanup(func() { l.Close() })

		port := l.Addr().(*net.TCPAddr).Port
		mnt := t.TempDir()
		b, err := exec.Command("mount_nfs", "-o", fmt.Sprintf("vers=3,tcp,port=%d,mountport=%d,nolocks,noresvport,locallocks,soft,timeo=10", port, port), "127.0.0.1:/", mnt).CombinedOutput()
		require.NoError(t, err, string(b))
		t.Cleanup(func() {
			if b, err := exec.Command("umount", "-f", mnt).CombinedOutput(); err != nil {
				t.Logf("unable to unmount %s: %v\n%s", mnt, err, b)
			}
		})
		return mnt, upperdir
	}

	var (
		console bytes.Buffer
	)

	archive := image(t)
	mnt, upperdir := mounted(t, archive)
	vm, err := egvm.Boot(egvm.Machine{
		Kernel:  filepath.Join(alpine, "Image"),
		Initrd:  initramfs(t),
		Cmdline: "console=hvc0 quiet",
		CPUs:    2,
		Memory:  512 << 20,
		Shares:  []egvm.Share{{Tag: egvm.Root, Dir: mnt}},
	}, &console)
	if err != nil && strings.Contains(err.Error(), "entitlement") {
		t.Skipf("the test binary is not signed with vz.entitlements: %v", err)
	}
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, vm.Close()) })

	run := func(t *testing.T, ctx context.Context, stdin io.Reader, cmd string, args ...string) (int32, string, string) {
		var (
			stdout bytes.Buffer
			stderr bytes.Buffer
		)

		code, err := vm.Exec(ctx, &execproxy.ExecRequest{Cmd: cmd, Arguments: args, Dir: "/", Environment: []string{"PATH=/bin"}}, stdin, &stdout, &stderr)
		require.NoError(t, err, console.String())
		return code, stdout.String(), stderr.String()
	}

	t.Run("runs the image's linux binaries with the egiso as their root", func(t *testing.T) {
		code, stdout, _ := run(t, t.Context(), nil, "sh", "-c", "cat /etc/hostname && uname -s")
		require.Equal(t, int32(0), code)
		require.Equal(t, "egvm\nLinux\n", stdout)
	})

	t.Run("writes go to the upper and leave the image as it is", func(t *testing.T) {
		code, _, stderr := run(t, t.Context(), nil, "sh", "-c", "echo written > /etc/written && mkdir -p /srv && echo data > /srv/data")
		require.Equal(t, int32(0), code, stderr)

		b, err := os.ReadFile(filepath.Join(upperdir, "etc", "written"))
		require.NoError(t, err)
		require.Equal(t, "written\n", string(b))
		b, err = os.ReadFile(filepath.Join(upperdir, "srv", "data"))
		require.NoError(t, err)
		require.Equal(t, "data\n", string(b))
		_, err = fs.Stat(archive, "etc/written")
		require.ErrorIs(t, err, fs.ErrNotExist)
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

		code, err := vm.Exec(t.Context(), &execproxy.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "pwd; echo $EGVM"}, Dir: "/etc", Environment: []string{"PATH=/bin", "EGVM=set"}}, nil, &stdout, io.Discard)
		require.NoError(t, err)
		require.Equal(t, int32(0), code)
		require.Equal(t, "/etc\nset\n", stdout.String())
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

		_, err := vm.Exec(ctx, &execproxy.ExecRequest{Cmd: "sh", Arguments: []string{"-c", "sleep 1; echo late > /tmp/late"}, Dir: "/", Environment: []string{"PATH=/bin"}}, nil, io.Discard, io.Discard)
		require.ErrorIs(t, err, context.DeadlineExceeded)

		time.Sleep(1500 * time.Millisecond)
		_, err = os.Stat(filepath.Join(upperdir, "tmp", "late"))
		require.ErrorIs(t, err, fs.ErrNotExist)
	})
}
