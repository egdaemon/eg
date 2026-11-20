package tarx_test

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg/internal/tarx"
	"github.com/stretchr/testify/require"
)

func TestSize(t *testing.T) {
	t.Run("sums the sizes of every entry", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested"), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), bytes.Repeat([]byte("a"), 10), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "nested", "b.txt"), bytes.Repeat([]byte("b"), 32), 0600))

		single := filepath.Join(t.TempDir(), "c.txt")
		require.NoError(t, os.WriteFile(single, bytes.Repeat([]byte("c"), 7), 0600))

		buf := bytes.NewBuffer(nil)
		require.NoError(t, tarx.Pack(buf, dir, single))

		n, err := tarx.Size(t.Context(), buf)
		require.NoError(t, err)
		require.Equal(t, int64(49), n)
	})

	t.Run("an empty archive is zero", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		require.NoError(t, tarx.Pack(buf))

		n, err := tarx.Size(t.Context(), buf)
		require.NoError(t, err)
		require.Zero(t, n)
	})

	t.Run("a reader that is not an archive fails", func(t *testing.T) {
		_, err := tarx.Size(t.Context(), bytes.NewBufferString("not an archive"))
		require.Error(t, err)
	})
}

func TestEntries(t *testing.T) {
	type entry struct {
		name string
		data string
	}

	archive := func(t *testing.T, entries ...entry) *bytes.Buffer {
		var (
			buf bytes.Buffer
		)

		w := tar.NewWriter(&buf)
		for _, e := range entries {
			require.NoError(t, tarx.WriteFileToArchive(w, &tar.Header{Typeflag: tar.TypeReg, Name: e.name, Mode: 0644, Size: int64(len(e.data))}, strings.NewReader(e.data)))
		}
		require.NoError(t, w.Close())

		return &buf
	}

	t.Run("yields each header with the reader at its contents", func(t *testing.T) {
		var (
			found []entry
		)

		tr := tar.NewReader(archive(t, entry{name: "a.txt", data: "a"}, entry{name: "b.txt", data: "bb"}))
		entries := tarx.Entries(tr)
		for header := range entries.Each(t.Context()) {
			data, err := io.ReadAll(tr)
			require.NoError(t, err)
			found = append(found, entry{name: header.Name, data: string(data)})
		}
		require.NoError(t, entries.Err())
		require.Equal(t, []entry{{name: "a.txt", data: "a"}, {name: "b.txt", data: "bb"}}, found)
	})

	t.Run("stops when the consumer stops", func(t *testing.T) {
		var (
			found []string
		)

		entries := tarx.Entries(tar.NewReader(archive(t, entry{name: "a.txt", data: "a"}, entry{name: "b.txt", data: "b"})))
		for header := range entries.Each(t.Context()) {
			found = append(found, header.Name)
			break
		}
		require.NoError(t, entries.Err())
		require.Equal(t, []string{"a.txt"}, found)
	})

	t.Run("an empty archive yields nothing", func(t *testing.T) {
		entries := tarx.Entries(tar.NewReader(archive(t)))
		for range entries.Each(t.Context()) {
			require.FailNow(t, "expected no entries")
		}
		require.NoError(t, entries.Err())
	})

	t.Run("a truncated archive fails", func(t *testing.T) {
		buf := archive(t, entry{name: "a.txt", data: strings.Repeat("a", 1024)})
		entries := tarx.Entries(tar.NewReader(bytes.NewReader(buf.Bytes()[:600])))
		for range entries.Each(t.Context()) {
		}
		require.Error(t, entries.Err())
	})
}

func TestWriteFileToArchive(t *testing.T) {
	t.Run("writes the header and its contents", func(t *testing.T) {
		var (
			buf bytes.Buffer
		)

		w := tar.NewWriter(&buf)
		require.NoError(t, tarx.WriteFileToArchive(w, &tar.Header{Typeflag: tar.TypeReg, Name: "etc/hostname", Mode: 0644, Size: 2}, strings.NewReader("eg")))
		require.NoError(t, w.Close())

		r := tar.NewReader(&buf)
		header, err := r.Next()
		require.NoError(t, err)
		require.Equal(t, "etc/hostname", header.Name)
		require.Equal(t, int64(0644), header.Mode)
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "eg", string(data))
	})

	t.Run("contents longer than the header's size fail", func(t *testing.T) {
		w := tar.NewWriter(io.Discard)
		require.ErrorIs(t, tarx.WriteFileToArchive(w, &tar.Header{Typeflag: tar.TypeReg, Name: "a.txt", Size: 1}, strings.NewReader("ab")), tar.ErrWriteTooLong)
	})

	t.Run("a header the writer rejects fails", func(t *testing.T) {
		w := tar.NewWriter(io.Discard)
		require.NoError(t, w.Close())
		require.ErrorIs(t, tarx.WriteFileToArchive(w, &tar.Header{Typeflag: tar.TypeReg, Name: "a.txt"}, strings.NewReader("")), tar.ErrWriteAfterClose)
	})
}
