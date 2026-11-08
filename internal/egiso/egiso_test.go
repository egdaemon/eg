package egiso_test

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"testing/fstest"
	"time"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/egdaemon/eg/internal/egiso"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

// image builds an egiso of a small root filesystem and the extra entries, with 1 KiB chunks so the
// large file spans many.
func image(t *testing.T, extra ...*tar.Header) (blob []byte, toc digest.Digest, large []byte) {
	var (
		buf bytes.Buffer
		out bytes.Buffer
	)

	large = make([]byte, 10*1024+7)
	_, err := rand.Read(large)
	require.NoError(t, err)

	tw := tar.NewWriter(&buf)
	write := func(h *tar.Header, body []byte) {
		h.ModTime = time.Unix(1700000000, 0)
		h.Size = int64(len(body))
		require.NoError(t, tw.WriteHeader(h))
		_, err := tw.Write(body)
		require.NoError(t, err)
	}
	write(&tar.Header{Typeflag: tar.TypeDir, Name: "etc/", Mode: 0755}, nil)
	write(&tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644, Uid: 0, Gid: 0, Uname: "root", Gname: "root"}, []byte("egd\n"))
	write(&tar.Header{Typeflag: tar.TypeDir, Name: "usr/", Mode: 0755}, nil)
	write(&tar.Header{Typeflag: tar.TypeDir, Name: "usr/bin/", Mode: 0755}, nil)
	write(&tar.Header{Typeflag: tar.TypeReg, Name: "usr/bin/tool", Mode: 0755}, []byte("#!/bin/sh\n"))
	write(&tar.Header{Typeflag: tar.TypeSymlink, Name: "bin", Linkname: "usr/bin"}, nil)
	write(&tar.Header{Typeflag: tar.TypeDir, Name: "home/", Mode: 0755}, nil)
	write(&tar.Header{Typeflag: tar.TypeDir, Name: "home/egd/", Mode: 0700, Uid: 1000, Gid: 1000, Uname: "egd", Gname: "egd"}, nil)
	write(&tar.Header{Typeflag: tar.TypeReg, Name: "home/egd/large", Mode: 0600, Uid: 1000, Gid: 1000, Uname: "egd", Gname: "egd"}, large)
	write(&tar.Header{Typeflag: tar.TypeLink, Name: "home/egd/hard", Linkname: "home/egd/large"}, nil)
	write(&tar.Header{Typeflag: tar.TypeSymlink, Name: "home/egd/hosts", Linkname: "/etc/hostname"}, nil)
	write(&tar.Header{Typeflag: tar.TypeSymlink, Name: "home/egd/up", Linkname: "../../../../etc"}, nil)
	for _, h := range extra {
		write(h, nil)
	}
	require.NoError(t, tw.Close())

	toc, err = egiso.Build(io.NewSectionReader(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len())), &out, estargz.WithChunkSize(1024))
	require.NoError(t, err)
	return out.Bytes(), toc, large
}

func section(b []byte) *io.SectionReader {
	return io.NewSectionReader(bytes.NewReader(b), 0, int64(len(b)))
}

func TestArchive(t *testing.T) {
	blob, toc, large := image(t)
	archive, err := egiso.New(section(blob), toc)
	require.NoError(t, err)

	t.Run("is a valid fs.FS", func(t *testing.T) {
		require.NoError(t, fstest.TestFS(archive, "etc/hostname", "usr/bin/tool", "home/egd/large", "home/egd/hard", "home/egd/hosts", "bin"))
	})

	t.Run("reads a file", func(t *testing.T) {
		b, err := fs.ReadFile(archive, "etc/hostname")
		require.NoError(t, err)
		require.Equal(t, "egd\n", string(b))
	})

	t.Run("reads a file that spans many chunks", func(t *testing.T) {
		b, err := fs.ReadFile(archive, "home/egd/large")
		require.NoError(t, err)
		require.Equal(t, large, b)
	})

	t.Run("reads a hardlink as the file it links to", func(t *testing.T) {
		b, err := fs.ReadFile(archive, "home/egd/hard")
		require.NoError(t, err)
		require.Equal(t, large, b)
	})

	t.Run("follows a relative symlink in a directory", func(t *testing.T) {
		b, err := fs.ReadFile(archive, "bin/tool")
		require.NoError(t, err)
		require.Equal(t, "#!/bin/sh\n", string(b))
	})

	t.Run("follows an absolute symlink from the archive's root", func(t *testing.T) {
		b, err := fs.ReadFile(archive, "home/egd/hosts")
		require.NoError(t, err)
		require.Equal(t, "egd\n", string(b))
	})

	t.Run("stops a symlink's parent directories at the root", func(t *testing.T) {
		b, err := fs.ReadFile(archive, "home/egd/up/hostname")
		require.NoError(t, err)
		require.Equal(t, "egd\n", string(b))
	})

	t.Run("names a symlink by its own name", func(t *testing.T) {
		info, err := fs.Stat(archive, "bin")
		require.NoError(t, err)
		require.Equal(t, "bin", info.Name())
		require.True(t, info.IsDir())
	})

	t.Run("describes a symlink without following it", func(t *testing.T) {
		info, err := fs.Lstat(archive, "bin")
		require.NoError(t, err)
		require.Equal(t, fs.ModeSymlink, info.Mode().Type())

		target, err := fs.ReadLink(archive, "bin")
		require.NoError(t, err)
		require.Equal(t, "usr/bin", target)
	})

	t.Run("reports the owner the image records", func(t *testing.T) {
		info, err := fs.Stat(archive, "home/egd/large")
		require.NoError(t, err)
		e := info.Sys().(*estargz.TOCEntry)
		require.Equal(t, 1000, e.UID)
		require.Equal(t, 1000, e.GID)
		require.Equal(t, "egd", e.Uname)
		require.Equal(t, "egd", e.Gname)
		require.Equal(t, fs.FileMode(0600), info.Mode().Perm())
	})

	t.Run("lists a directory in name order", func(t *testing.T) {
		entries, err := fs.ReadDir(archive, "home/egd")
		require.NoError(t, err)
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		require.Equal(t, []string{"hard", "hosts", "large", "up"}, names)
	})

	t.Run("reports a missing file", func(t *testing.T) {
		_, err := archive.Open("etc/missing")
		require.ErrorIs(t, err, fs.ErrNotExist)
	})

	t.Run("reports a file used as a directory", func(t *testing.T) {
		_, err := archive.Open("etc/hostname/x")
		require.ErrorIs(t, err, syscall.ENOTDIR)
	})

	t.Run("refuses a symlink loop", func(t *testing.T) {
		blob, toc, _ := image(t, &tar.Header{Typeflag: tar.TypeSymlink, Name: "loop", Linkname: "loop"})
		archive, err := egiso.New(section(blob), toc)
		require.NoError(t, err)

		_, err = archive.Open("loop")
		require.ErrorIs(t, err, syscall.ELOOP)
	})

	t.Run("refuses an invalid path", func(t *testing.T) {
		_, err := archive.Open("/etc/hostname")
		require.ErrorIs(t, err, fs.ErrInvalid)
	})
}

func TestNew(t *testing.T) {
	blob, toc, _ := image(t)

	t.Run("refuses an archive whose table of contents does not match", func(t *testing.T) {
		_, err := egiso.New(section(blob), digest.FromString("other"))
		require.ErrorContains(t, err, "unable to verify the egiso")
	})

	t.Run("refuses a tampered chunk", func(t *testing.T) {
		archive, err := egiso.New(section(blob), toc)
		require.NoError(t, err)
		info, err := fs.Stat(archive, "home/egd/large")
		require.NoError(t, err)

		// flip a byte in the middle of the large file's compressed chunks: the table of contents
		// still matches, but the chunk holding it doesn't.
		tampered := bytes.Clone(blob)
		e := info.Sys().(*estargz.TOCEntry)
		tampered[e.Offset+40] ^= 0xff
		archive, err = egiso.New(section(tampered), toc)
		require.NoError(t, err)

		_, err = fs.ReadFile(archive, "home/egd/large")
		require.Error(t, err)
	})
}

func TestOpen(t *testing.T) {
	blob, toc, _ := image(t)
	path := filepath.Join(t.TempDir(), "root.egiso")
	require.NoError(t, os.WriteFile(path, blob, 0600))

	t.Run("opens a local archive", func(t *testing.T) {
		archive, err := egiso.Open(t.Context(), path, toc)
		require.NoError(t, err)
		defer archive.Close()

		b, err := fs.ReadFile(archive, "etc/hostname")
		require.NoError(t, err)
		require.Equal(t, "egd\n", string(b))
	})

	t.Run("refuses a location that is not a path or an https URL", func(t *testing.T) {
		_, err := egiso.Open(t.Context(), "http://example.com/root.egiso", toc)
		require.ErrorContains(t, err, "unsupported egiso location")
	})
}

func TestRemote(t *testing.T) {
	blob, toc, large := image(t)

	t.Run("reads only the ranges it needs", func(t *testing.T) {
		var (
			served int64
		)
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cw := &counter{ResponseWriter: w}
			http.ServeContent(cw, r, "root.egiso", time.Time{}, bytes.NewReader(blob))
			served += cw.n
		}))
		defer srv.Close()

		sr, err := egiso.Remote(t.Context(), srv.Client(), srv.URL+"/root.egiso")
		require.NoError(t, err)
		archive, err := egiso.New(sr, toc)
		require.NoError(t, err)

		b, err := fs.ReadFile(archive, "etc/hostname")
		require.NoError(t, err)
		require.Equal(t, "egd\n", string(b))
		require.Less(t, served, int64(len(blob)), "the large file was not fetched")

		b, err = fs.ReadFile(archive, "home/egd/large")
		require.NoError(t, err)
		require.Equal(t, large, b)
	})

	t.Run("refuses a server that ignores ranges", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1024")
			if r.Method == http.MethodGet {
				w.Write(make([]byte, 1024))
			}
		}))
		defer srv.Close()

		sr, err := egiso.Remote(t.Context(), srv.Client(), srv.URL)
		require.NoError(t, err)
		_, err = egiso.New(sr, toc)
		require.ErrorContains(t, err, "does not serve ranges")
	})
}

// counter counts the body bytes a handler writes.
type counter struct {
	http.ResponseWriter
	n int64
}

func (t *counter) Write(p []byte) (int, error) {
	n, err := t.ResponseWriter.Write(p)
	t.n += int64(n)
	return n, err
}
