package egvm

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// connected returns both ends of a connected packet socket pair.
func connected(t *testing.T) (*net.UnixConn, *net.UnixConn) {
	var (
		conns [2]*net.UnixConn
		fds   [2]int
		c     net.Conn
		err   error
	)

	fds, err = unix.Socketpair(unix.AF_UNIX, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	require.NoError(t, err)
	for i, fd := range fds {
		f := os.NewFile(uintptr(fd), "conn")
		c, err = net.FileConn(f)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		conns[i] = c.(*net.UnixConn)
		t.Cleanup(func() { conns[i].Close() })
	}
	return conns[0], conns[1]
}

func TestMachineOpen(t *testing.T) {
	// machine mounts two host directories, the second within the first, and writes a file in each.
	machine := func(t *testing.T) (Machine, string, string) {
		outer, inner := t.TempDir(), t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(outer, "file"), []byte("outer"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(inner, "file"), []byte("inner"), 0600))
		return New(nil, OptionMountOwned(outer, "/eg.mnt"), OptionMountReadOnly(inner, "/eg.mnt/inner")), outer, inner
	}

	read := func(t *testing.T, f *os.File) string {
		defer f.Close()
		b, err := os.ReadFile(f.Name())
		require.NoError(t, err)
		return string(b)
	}

	t.Run("opens a file by the path the machine sees it at", func(t *testing.T) {
		m, _, _ := machine(t)
		f, err := m.open("/eg.mnt/file")
		require.NoError(t, err)
		require.Equal(t, "outer", read(t, f))
	})

	t.Run("opens a file from the deepest directory holding it", func(t *testing.T) {
		m, _, _ := machine(t)
		f, err := m.open("/eg.mnt/inner/file")
		require.NoError(t, err)
		require.Equal(t, "inner", read(t, f))
	})

	t.Run("refuses a path within no directory the host mounts", func(t *testing.T) {
		m, _, _ := machine(t)
		_, err := m.open("/etc/passwd")
		require.ErrorContains(t, err, "not within a directory the host mounts")
	})

	t.Run("refuses a path climbing out of the directory it names", func(t *testing.T) {
		m, _, _ := machine(t)
		_, err := m.open("/eg.mnt/../etc/passwd")
		require.ErrorContains(t, err, "not within a directory the host mounts")
	})

	t.Run("opens a file by an alias when its path is within no directory the host mounts", func(t *testing.T) {
		m, _, _ := machine(t)
		f, err := m.open("/workload/file", "/eg.mnt/file")
		require.NoError(t, err)
		require.Equal(t, "outer", read(t, f))
	})

	t.Run("refuses a file none of whose aliases are within a directory the host mounts", func(t *testing.T) {
		m, _, _ := machine(t)
		_, err := m.open("/workload/file", "/srv/file")
		require.ErrorContains(t, err, "not within a directory the host mounts")
	})

	t.Run("refuses a relative path", func(t *testing.T) {
		m, _, _ := machine(t)
		_, err := m.open("eg.mnt/file")
		require.ErrorContains(t, err, "not an absolute path")
	})

	t.Run("refuses a symlink leading out of the directory", func(t *testing.T) {
		m, outer, _ := machine(t)
		require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(outer, "escape")))
		_, err := m.open("/eg.mnt/escape")
		require.Error(t, err)
	})
}

func TestReceive(t *testing.T) {
	t.Run("reads a request and the descriptor sent with it", func(t *testing.T) {
		a, b := connected(t)
		f, err := os.Open(os.DevNull)
		require.NoError(t, err)
		defer f.Close()

		require.NoError(t, send(a, rootrequest{Egiso: "/eg.mnt/root.egiso#sha256:00"}, f))
		req, dev, err := receive(b)
		require.NoError(t, err)
		require.NotNil(t, dev)
		defer dev.Close()
		require.Equal(t, "/eg.mnt/root.egiso#sha256:00", req.Egiso)
	})

	t.Run("reads a request sent without a descriptor", func(t *testing.T) {
		a, b := connected(t)
		require.NoError(t, send(a, rootrequest{Glob: "etc/ssh/*"}, nil))
		req, dev, err := receive(b)
		require.NoError(t, err)
		require.Nil(t, dev)
		require.Equal(t, "etc/ssh/*", req.Glob)
	})

	t.Run("ends once the other end hangs up", func(t *testing.T) {
		a, b := connected(t)
		require.NoError(t, a.Close())
		_, _, err := receive(b)
		require.Error(t, err)
	})
}

func TestAliases(t *testing.T) {
	t.Run("never names the path itself", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(path, nil, 0600))
		found, err := aliases(path)
		require.NoError(t, err)
		require.NotContains(t, found, path)
	})

	t.Run("names the path a bind mount shows the file at", func(t *testing.T) {
		var (
			src = t.TempDir()
			dst = t.TempDir()
		)
		if os.Getuid() != 0 {
			t.Skip("bind mounting needs root")
		}
		require.NoError(t, os.WriteFile(filepath.Join(src, "file"), nil, 0600))
		require.NoError(t, unix.Mount(src, dst, "", unix.MS_BIND, ""))
		t.Cleanup(func() { unix.Unmount(dst, unix.MNT_DETACH) })

		found, err := aliases(filepath.Join(dst, "file"))
		require.NoError(t, err)
		require.Contains(t, found, filepath.Join(src, "file"))
	})
}

func TestNested(t *testing.T) {
	t.Run("refuses a descriptor that is not /dev/fuse", func(t *testing.T) {
		var (
			vm = &VM{}
		)
		f, err := os.Open(os.DevNull)
		require.NoError(t, err)
		defer f.Close()

		_, err = vm.nested(New(nil), "/eg.mnt/root.egiso#sha256:00", nil, f)
		require.ErrorContains(t, err, "not /dev/fuse")
	})
}

func TestHost(t *testing.T) {
	// answer replies to each request the host's other end receives with reply, recording it in got.
	answer := func(t *testing.T, conn *net.UnixConn, reply rootreply, got chan<- rootrequest) {
		go func() {
			for {
				req, dev, err := receive(conn)
				if err != nil {
					close(got)
					return
				}
				if dev != nil {
					dev.Close()
				}
				got <- req
				if send(conn, reply, nil) != nil {
					return
				}
			}
		}()
	}

	t.Run("asks to serve the egiso at the path its links lead to", func(t *testing.T) {
		var (
			got = make(chan rootrequest, 1)
			dir = t.TempDir()
		)
		a, b := connected(t)
		answer(t, b, rootreply{}, got)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "root.egiso"), nil, 0600))
		link := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink(dir, link))
		f, err := os.Open(os.DevNull)
		require.NoError(t, err)
		defer f.Close()

		require.NoError(t, (&host{conn: a}).serve(f, filepath.Join(link, "root.egiso")+"#sha256:00"))
		resolved, err := filepath.EvalSymlinks(dir)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(resolved, "root.egiso")+"#sha256:00", (<-got).Egiso)
	})

	t.Run("asks to serve an https egiso as it is named", func(t *testing.T) {
		var (
			got = make(chan rootrequest, 1)
		)
		a, b := connected(t)
		answer(t, b, rootreply{}, got)
		f, err := os.Open(os.DevNull)
		require.NoError(t, err)
		defer f.Close()

		require.NoError(t, (&host{conn: a}).serve(f, "https://example.com/root.egiso#sha256:00"))
		require.Equal(t, "https://example.com/root.egiso#sha256:00", (<-got).Egiso)
	})

	t.Run("fails when the host does not serve the root", func(t *testing.T) {
		var (
			got = make(chan rootrequest, 1)
		)
		a, b := connected(t)
		answer(t, b, rootreply{Error: "the machine sent a descriptor that is not /dev/fuse"}, got)
		f, err := os.Open(os.DevNull)
		require.NoError(t, err)
		defer f.Close()

		require.ErrorContains(t, (&host{conn: a}).serve(f, "https://example.com/root.egiso#sha256:00"), "not /dev/fuse")
	})

	t.Run("globs and reads the root the host serves", func(t *testing.T) {
		var (
			got = make(chan rootrequest, 2)
		)
		a, b := connected(t)
		answer(t, b, rootreply{Names: []string{"etc/ssh/ssh_host_ed25519_key.pub"}, Data: []byte("key")}, got)
		h := &host{conn: a}

		names, err := fs.Glob(h, "etc/ssh/ssh_host_*_key.pub")
		require.NoError(t, err)
		require.Equal(t, []string{"etc/ssh/ssh_host_ed25519_key.pub"}, names)
		require.Equal(t, "etc/ssh/ssh_host_*_key.pub", (<-got).Glob)

		b2, err := fs.ReadFile(h, "etc/ssh/ssh_host_ed25519_key.pub")
		require.NoError(t, err)
		require.Equal(t, "key", string(b2))
		require.Equal(t, "etc/ssh/ssh_host_ed25519_key.pub", (<-got).Read)
	})

	t.Run("reports a file the root does not hold", func(t *testing.T) {
		var (
			got = make(chan rootrequest, 1)
		)
		a, b := connected(t)
		answer(t, b, rootreply{Error: "file does not exist"}, got)

		_, err := fs.ReadFile(&host{conn: a}, "etc/missing")
		require.ErrorContains(t, err, "file does not exist")
	})
}
