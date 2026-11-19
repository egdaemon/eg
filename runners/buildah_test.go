package runners

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

type tarentry struct {
	name string
	data string
}

// gzlayer writes the entries as a gzipped tar archive.
func gzlayer(t *testing.T, entries ...tarentry) []byte {
	var (
		buf bytes.Buffer
	)

	gz := gzip.NewWriter(&buf)
	w := tar.NewWriter(gz)
	for _, e := range entries {
		require.NoError(t, w.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: e.name, Mode: 0644, Size: int64(len(e.data))}))
		_, err := w.Write([]byte(e.data))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, gz.Close())

	return buf.Bytes()
}

// untar lists the entries of the tar archive in order.
func untar(t *testing.T, archive []byte) (entries []tarentry) {
	var (
		hdr  *tar.Header
		data []byte
		err  error
	)

	r := tar.NewReader(bytes.NewReader(archive))
	for hdr, err = r.Next(); err != io.EOF; hdr, err = r.Next() {
		require.NoError(t, err)
		data, err = io.ReadAll(r)
		require.NoError(t, err)
		entries = append(entries, tarentry{name: hdr.Name, data: string(data)})
	}

	return entries
}

func TestSquashed(t *testing.T) {
	// blob writes b into the layout's blobs and describes it.
	blob := func(t *testing.T, layout string, b []byte) ocispec.Descriptor {
		d := digest.FromBytes(b)
		dir := filepath.Join(layout, ocispec.ImageBlobsDir, d.Algorithm().String())
		require.NoError(t, os.MkdirAll(dir, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, d.Encoded()), b, 0600))
		return ocispec.Descriptor{Digest: d, Size: int64(len(b))}
	}

	document := func(t *testing.T, layout string, v any) ocispec.Descriptor {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		return blob(t, layout, b)
	}

	// image writes a layout of images, each with the given number of layers and the environment.
	image := func(t *testing.T, images int, layers int, env ...string) (layout string, first ocispec.Descriptor) {
		var (
			index ocispec.Index
		)

		layout = t.TempDir()
		for range images {
			manifest := ocispec.Manifest{Config: document(t, layout, ocispec.Image{Config: ocispec.ImageConfig{Env: env}})}
			for i := range layers {
				manifest.Layers = append(manifest.Layers, blob(t, layout, gzlayer(t, tarentry{name: "layer", data: string(rune('a' + i))})))
			}
			index.Manifests = append(index.Manifests, document(t, layout, manifest))
			if len(manifest.Layers) > 0 && first.Digest == "" {
				first = manifest.Layers[0]
			}
		}
		encoded, err := json.Marshal(index)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(layout, ocispec.ImageIndexFile), encoded, 0600))

		return layout, first
	}

	t.Run("returns the only layer and the configuration's environment", func(t *testing.T) {
		layout, first := image(t, 1, 1, "PATH=/usr/bin", "GREETING=hello")

		layer, environ, err := squashed(layout)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(layout, ocispec.ImageBlobsDir, "sha256", first.Digest.Encoded()), layer)
		require.Equal(t, []string{"PATH=/usr/bin", "GREETING=hello"}, environ)
	})

	t.Run("an image with no environment returns none", func(t *testing.T) {
		layout, _ := image(t, 1, 1)

		_, environ, err := squashed(layout)
		require.NoError(t, err)
		require.Empty(t, environ)
	})

	t.Run("an image of several layers fails", func(t *testing.T) {
		layout, _ := image(t, 1, 2)

		_, _, err := squashed(layout)
		require.ErrorContains(t, err, "found 2")
	})

	t.Run("a layout of several images fails", func(t *testing.T) {
		layout, _ := image(t, 2, 1)

		_, _, err := squashed(layout)
		require.ErrorContains(t, err, "found 2")
	})

	t.Run("a missing index fails", func(t *testing.T) {
		_, _, err := squashed(t.TempDir())
		require.ErrorContains(t, err, "unable to read the image's index")
	})
}

func TestWithenviron(t *testing.T) {
	// rewrite writes the entries as a gzipped layer and returns withenviron's archive of it.
	rewrite := func(t *testing.T, environ []string, entries ...tarentry) []byte {
		var (
			buf bytes.Buffer
		)

		layer := filepath.Join(t.TempDir(), "layer")
		require.NoError(t, os.WriteFile(layer, gzlayer(t, entries...), 0600))
		require.NoError(t, withenviron(layer, environ, &buf))
		return buf.Bytes()
	}

	t.Run("appends the environment after the layer's entries", func(t *testing.T) {
		archive := rewrite(t, []string{"PATH=/usr/bin", "GREETING=hello"}, tarentry{name: "etc/hostname", data: "eg"}, tarentry{name: "usr/bin/hello", data: "binary"})
		require.Equal(t, []tarentry{
			{name: "etc/hostname", data: "eg"},
			{name: "usr/bin/hello", data: "binary"},
			{name: "etc/environment", data: "PATH=/usr/bin\nGREETING=hello\n"},
		}, untar(t, archive))
	})

	t.Run("replaces the layer's own environment, with or without a leading ./", func(t *testing.T) {
		archive := rewrite(t, []string{"GREETING=hello"}, tarentry{name: "./etc/environment", data: "OLD=1\n"}, tarentry{name: "etc/environment", data: "OLDER=1\n"}, tarentry{name: "etc/hosts", data: "localhost"})
		require.Equal(t, []tarentry{
			{name: "etc/hosts", data: "localhost"},
			{name: "etc/environment", data: "GREETING=hello\n"},
		}, untar(t, archive))
	})

	t.Run("no environment writes an empty one", func(t *testing.T) {
		archive := rewrite(t, nil, tarentry{name: "etc/hostname", data: "eg"})
		require.Equal(t, []tarentry{
			{name: "etc/hostname", data: "eg"},
			{name: "etc/environment", data: ""},
		}, untar(t, archive))
	})

	t.Run("a layer that is not gzipped fails", func(t *testing.T) {
		var (
			buf bytes.Buffer
		)

		layer := filepath.Join(t.TempDir(), "layer")
		require.NoError(t, os.WriteFile(layer, []byte("not a layer"), 0600))
		require.ErrorContains(t, withenviron(layer, nil, &buf), "unable to decompress the image's layer")
	})

	t.Run("a missing layer fails", func(t *testing.T) {
		var (
			buf bytes.Buffer
		)

		require.ErrorContains(t, withenviron(filepath.Join(t.TempDir(), "missing"), nil, &buf), "unable to open the image's layer")
	})
}

func TestBuildEgiso(t *testing.T) {
	if _, err := exec.LookPath("buildah"); err != nil {
		t.Skip("buildah is not installed")
	}

	// context writes a Containerfile that needs no network into a build context.
	buildcontext := func(t *testing.T) (dir string) {
		dir = t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Containerfile"), []byte("FROM scratch\nCOPY hello.txt /hello.txt\nENV GREETING=hello\n"), 0644))
		return dir
	}

	t.Run("builds an egiso of the image with its environment", func(t *testing.T) {
		dir := buildcontext(t)

		uri, err := BuildEgiso(t.Context(), t.TempDir(), dir, filepath.Join(dir, "Containerfile"))
		require.NoError(t, err)

		archive, err := egiso.OpenURI(t.Context(), http.DefaultClient, uri)
		require.NoError(t, err)
		defer archive.Close()

		hello, err := fs.ReadFile(archive, "hello.txt")
		require.NoError(t, err)
		require.Equal(t, "hello world\n", string(hello))

		environment, err := fs.ReadFile(archive, "etc/environment")
		require.NoError(t, err)
		require.Contains(t, string(environment), "GREETING=hello\n")
	})

	t.Run("an unchanged image reuses its egiso", func(t *testing.T) {
		dir := buildcontext(t)
		cache := t.TempDir()

		first, err := BuildEgiso(t.Context(), cache, dir, filepath.Join(dir, "Containerfile"))
		require.NoError(t, err)

		second, err := BuildEgiso(t.Context(), cache, dir, filepath.Join(dir, "Containerfile"))
		require.NoError(t, err)
		require.Equal(t, first, second)
	})

	t.Run("a Containerfile that fails to build fails", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "Containerfile"), []byte("FROM scratch\nCOPY missing.txt /missing.txt\n"), 0644))

		_, err := BuildEgiso(t.Context(), t.TempDir(), dir, filepath.Join(dir, "Containerfile"))
		require.ErrorContains(t, err, "unable to build the image")
	})
}
