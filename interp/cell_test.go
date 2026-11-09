package interp

import (
	"archive/tar"
	"bytes"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/interp/interptest"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid/v5"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestCell(t *testing.T) {
	var (
		buf bytes.Buffer
		out bytes.Buffer
	)

	wasm := interptest.Build(t, "./interp/.fixtures/cell")

	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "etc/", Mode: 0755}))
	require.NoError(t, tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644, Size: 4}))
	_, err := tw.Write([]byte("egd\n"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	toc, err := egiso.Build(io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len())), &out)
	require.NoError(t, err)
	image := filepath.Join(t.TempDir(), "root.egiso")
	require.NoError(t, os.WriteFile(image, out.Bytes(), 0600))

	svc, err := grpc.NewClient("passthrough:///unused", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { svc.Close() })

	workspace := func(t *testing.T) workspaces.Context {
		return workspaces.Context{
			Root:           t.TempDir(),
			RuntimeDir:     t.TempDir(),
			CacheDir:       t.TempDir(),
			CacheDirWazero: t.TempDir(),
			WorkspaceDir:   t.TempDir(),
			WorkingDir:     t.TempDir(),
		}
	}

	cell := func(t *testing.T, ws workspaces.Context, toc digest.Digest) error {
		return Cell(t.Context(), ws, uuid.Nil.String(), uuid.Nil.String(), svc, http.DefaultClient, wasm, image, toc)
	}

	t.Run("runs the module with the egiso as its root", func(t *testing.T) {
		ws := workspace(t)
		require.NoError(t, cell(t, ws, toc))

		b, err := os.ReadFile(filepath.Join(ws.WorkingDir, "hostname"))
		require.NoError(t, err)
		require.Equal(t, "egd\n", string(b))
	})

	t.Run("removes the upper after the run", func(t *testing.T) {
		ws := workspace(t)
		require.NoError(t, cell(t, ws, toc))

		matches, err := filepath.Glob(filepath.Join(ws.RuntimeDir, "cell.*"))
		require.NoError(t, err)
		require.Empty(t, matches)
	})

	t.Run("refuses an egiso whose table of contents does not match", func(t *testing.T) {
		require.ErrorContains(t, cell(t, workspace(t), digest.FromString("other")), "unable to verify the egiso")
	})

	// needs EG_TEST_ALPINE, as TestExec in internal/egvm describes, and the test binary signed with
	// internal/egvm/vz.entitlements.
	t.Run("runs the commands the module does not provide in a machine over the same root", func(t *testing.T) {
		var (
			buf       bytes.Buffer
			out       bytes.Buffer
			initramfs bytes.Buffer
		)

		alpine := os.Getenv("EG_TEST_ALPINE")
		if alpine == "" {
			t.Skip("EG_TEST_ALPINE is not set")
		}
		if b, _ := exec.Command("codesign", "-d", "--entitlements", "-", os.Args[0]).CombinedOutput(); !strings.Contains(string(b), "com.apple.security.virtualization") {
			t.Skip("the test binary is not signed with internal/egvm/vz.entitlements")
		}

		read := func(name string) []byte {
			b, err := os.ReadFile(name)
			require.NoError(t, err)
			return b
		}

		vminit := filepath.Join(t.TempDir(), "init")
		cmd := exec.Command("go", "build", "-o", vminit, "../internal/egvm/vminit")
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0")
		b, err := cmd.CombinedOutput()
		require.NoError(t, err, string(b))
		kernels, err := filepath.Glob(filepath.Join(alpine, "modules", "*", "kernel"))
		require.NoError(t, err)
		require.Len(t, kernels, 1)
		require.NoError(t, egvm.MachineInitramfs(&initramfs, read(vminit), kernels[0]))

		config := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", config)
		require.NoError(t, os.MkdirAll(filepath.Join(config, "eg", "egvm.d"), 0700))
		require.NoError(t, os.Symlink(filepath.Join(alpine, "Image"), filepath.Join(config, "eg", "egvm.d", "Image")))
		require.NoError(t, os.WriteFile(filepath.Join(config, "eg", "egvm.d", "initramfs"), initramfs.Bytes(), 0600))

		tw := tar.NewWriter(&buf)
		write := func(h *tar.Header, data []byte) {
			h.Size = int64(len(data))
			h.ModTime = time.Unix(1700000000, 0)
			require.NoError(t, tw.WriteHeader(h))
			_, err := tw.Write(data)
			require.NoError(t, err)
		}
		for _, dir := range []string{"bin/", "lib/", "etc/"} {
			write(&tar.Header{Typeflag: tar.TypeDir, Name: dir, Mode: 0755}, nil)
		}
		write(&tar.Header{Typeflag: tar.TypeReg, Name: "bin/busybox", Mode: 0755}, read(filepath.Join(alpine, "initrd", "bin", "busybox")))
		write(&tar.Header{Typeflag: tar.TypeSymlink, Name: "bin/sh", Linkname: "/bin/busybox", Mode: 0777}, nil)
		write(&tar.Header{Typeflag: tar.TypeSymlink, Name: "bin/uname", Linkname: "/bin/busybox", Mode: 0777}, nil)
		write(&tar.Header{Typeflag: tar.TypeReg, Name: "lib/ld-musl-aarch64.so.1", Mode: 0755}, read(filepath.Join(alpine, "initrd", "lib", "ld-musl-aarch64.so.1")))
		write(&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644}, []byte("egd\n"))
		require.NoError(t, tw.Close())

		toc, err := egiso.Build(io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len())), &out)
		require.NoError(t, err)
		image := filepath.Join(t.TempDir(), "root.egiso")
		require.NoError(t, os.WriteFile(image, out.Bytes(), 0600))

		// the module's command listens on unix sockets in the runtime directory, whose paths are limited to
		// about 100 bytes, more than test directories leave.
		ws := workspace(t)
		ws.RuntimeDir, err = os.MkdirTemp("/tmp", "cell")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(ws.RuntimeDir) })

		require.NoError(t, Cell(t.Context(), ws, uuid.Nil.String(), uuid.Nil.String(), svc, http.DefaultClient, wasm, image, toc, "CELL_MACHINE=1"))

		b, err = os.ReadFile(filepath.Join(ws.WorkingDir, "machine"))
		require.NoError(t, err)
		require.Equal(t, "Linux\n", string(b))
	})
}
