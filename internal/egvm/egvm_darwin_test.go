package egvm_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/stretchr/testify/require"
)

// TestRun boots a Linux machine whose root is an egiso served over NFS. It needs EG_TEST_ALPINE, a
// directory holding an uncompressed arm64 kernel (Image) and Alpine's initramfs unpacked (initrd),
// and the test binary signed with vz.entitlements:
//
//	go test -c -o egvm.test ./internal/egvm && codesign -f -s - --entitlements internal/egvm/vz.entitlements egvm.test && ./egvm.test -test.v
func TestRun(t *testing.T) {
	alpine := os.Getenv("EG_TEST_ALPINE")
	if alpine == "" {
		t.Skip("EG_TEST_ALPINE is not set")
	}
	initrd := filepath.Join(alpine, "initrd")
	modules, err := filepath.Glob(filepath.Join(initrd, "usr", "lib", "modules", "*", "kernel", "fs", "fuse"))
	require.NoError(t, err)
	require.Len(t, modules, 1)

	// initramfs holds vminit and the modules virtio-fs needs, loaded in name order.
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

		require.NoError(t, egvm.Initramfs(&buf,
			egvm.File{Name: "proc", Mode: fs.ModeDir | 0755},
			egvm.File{Name: "sys", Mode: fs.ModeDir | 0755},
			egvm.File{Name: "dev", Mode: fs.ModeDir | 0755},
			egvm.File{Name: "root", Mode: fs.ModeDir | 0755},
			egvm.File{Name: "modules", Mode: fs.ModeDir | 0755},
			egvm.File{Name: "modules/1.fuse.ko", Mode: 0644, Data: read(filepath.Join(modules[0], "fuse.ko"))},
			egvm.File{Name: "modules/2.virtiofs.ko", Mode: 0644, Data: read(filepath.Join(modules[0], "virtiofs.ko"))},
			egvm.File{Name: "init", Mode: 0755, Data: read(vminit)},
		))

		path := filepath.Join(t.TempDir(), "initramfs")
		require.NoError(t, os.WriteFile(path, buf.Bytes(), 0600))
		return path
	}

	// image is an egiso of Alpine's busybox and musl, with a script that reads and writes the root.
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
		write(&tar.Header{Typeflag: tar.TypeSymlink, Name: "bin/sh", Linkname: "/bin/busybox", Mode: 0777}, nil)
		write(&tar.Header{Typeflag: tar.TypeReg, Name: "lib/ld-musl-aarch64.so.1", Mode: 0755}, host("lib/ld-musl-aarch64.so.1"))
		write(&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644}, []byte("egvm\n"))
		write(&tar.Header{Typeflag: tar.TypeReg, Name: "egvm.sh", Mode: 0755}, []byte(strings.Join([]string{
			"/bin/busybox cat /etc/hostname",
			"/bin/busybox uname -s",
			"echo written > /etc/written",
			"/bin/busybox mkdir -p /srv && echo data > /srv/data",
			"/bin/busybox cat /srv/data",
		}, "\n")+"\n"))
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

	t.Run("runs the image's linux binaries with the egiso as their root", func(t *testing.T) {
		var (
			console bytes.Buffer
		)

		archive := image(t)
		mnt, upperdir := mounted(t, archive)
		machine := egvm.Machine{
			Kernel:  filepath.Join(alpine, "Image"),
			Initrd:  initramfs(t),
			Cmdline: "console=hvc0 quiet -- /bin/busybox sh /egvm.sh",
			CPUs:    1,
			Memory:  512 << 20,
			Shares:  []egvm.Share{{Tag: egvm.Root, Dir: mnt}},
		}

		err := egvm.Run(t.Context(), machine, &console)
		if err != nil && strings.Contains(err.Error(), "entitlement") {
			t.Skipf("the test binary is not signed with vz.entitlements: %v", err)
		}
		require.NoError(t, err, console.String())

		// the console ends its lines with \r\n.
		out := strings.ReplaceAll(console.String(), "\r\n", "\n")
		require.Contains(t, out, "egvm\n", out)
		require.Contains(t, out, "Linux\n", out)
		require.Contains(t, out, "data\n", out)
		require.Contains(t, out, "egvm: exit 0", out)

		b, err := os.ReadFile(filepath.Join(upperdir, "etc", "written"))
		require.NoError(t, err)
		require.Equal(t, "written\n", string(b))
		b, err = os.ReadFile(filepath.Join(upperdir, "srv", "data"))
		require.NoError(t, err)
		require.Equal(t, "data\n", string(b))
		_, err = fs.Stat(archive, "etc/written")
		require.ErrorIs(t, err, fs.ErrNotExist)
	})
}
