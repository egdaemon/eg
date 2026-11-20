package runners

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// BuildEgiso builds the Containerfile at path, in the context dir, with buildah, and returns the uri of
// an egiso of the image, as ImageEgiso does.
func BuildEgiso(ctx context.Context, cache, dir, path string, options ...string) (_ string, err error) {
	var (
		image string
	)

	if image, err = BuildImage(ctx, dir, path, options...); err != nil {
		return "", err
	}
	return ImageEgiso(ctx, cache, image)
}

// BuildImage builds the Containerfile at path, in the context dir, with buildah, and returns the image's id.
func BuildImage(ctx context.Context, dir, path string, options ...string) (_ string, err error) {
	var (
		iid *os.File
		id  []byte
		cmd *exec.Cmd
	)

	if iid, err = os.CreateTemp("", "eg.buildah.iid.*"); err != nil {
		return "", errorsx.Wrap(err, "unable to create the image id file")
	}
	defer func() { err = errorsx.Compact(err, os.Remove(iid.Name())) }()
	if err = iid.Close(); err != nil {
		return "", err
	}

	// --layers caches each step, and a fixed timestamp keeps an unchanged image's id, which keys the cache.
	cmd = exec.CommandContext(ctx, "buildah", append(append([]string{"build", "--layers", "--timestamp", "0", "--iidfile", iid.Name(), "-f", path}, options...), dir)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err = execx.MaybeRun(cmd); err != nil {
		return "", errorsx.Wrap(err, "unable to build the image")
	}

	if id, err = os.ReadFile(iid.Name()); err != nil {
		return "", errorsx.Wrap(err, "unable to identify the image")
	}
	return digest.Digest(strings.TrimSpace(string(id))).Encoded(), nil
}

// ImageEgiso returns the uri of an egiso of image, the name or id of an image in buildah's storage, squashed to
// a single layer, with the image's environment as its /etc/environment. It keeps the egiso of the latest image in
// cache, in a directory named for the image's id, so it squashes and converts one per image.
func ImageEgiso(ctx context.Context, cache, image string) (_ string, err error) {
	var (
		id        string
		container string
		squash    string
		layout    string
		layer     string
		environ   []string
		root      *os.File
		st        os.FileInfo
		out       *os.File
		toc       digest.Digest
		cached    []byte
		cmd       *exec.Cmd
	)

	if id, err = execx.String(ctx, "buildah", "inspect", "--type", "image", "--format", "{{.FromImageID}}", image); err != nil {
		return "", errorsx.Wrapf(err, "unable to identify the image: %s", image)
	}
	id = strings.TrimSpace(id)
	imagedir := filepath.Join(cache, id)
	location := filepath.Join(imagedir, "root.egiso")

	// the table of contents is written once its egiso is complete.
	if cached, err = os.ReadFile(filepath.Join(imagedir, "toc")); err == nil {
		return location + "#" + string(cached), nil
	}

	if err = os.MkdirAll(imagedir, 0700); err != nil {
		return "", errorsx.Wrap(err, "unable to create the egiso cache")
	}

	if layout, err = os.MkdirTemp(imagedir, "oci.*"); err != nil {
		return "", errorsx.Wrap(err, "unable to create the image's layout")
	}
	defer func() { err = errorsx.Compact(err, os.RemoveAll(layout)) }()

	// squashing includes the layers of the base image, so the egiso is one layer with no whiteouts. a
	// squash of an unchanged image differs from build to build, so only an image the cache misses is squashed.
	if container, err = execx.String(ctx, "buildah", "from", "--pull=never", id); err != nil {
		return "", errorsx.Wrap(err, "unable to create the image's container")
	}
	container = strings.TrimSpace(container)
	defer func() {
		errorsx.Log(errorsx.Wrapf(exec.Command("buildah", "rm", container).Run(), "unable to remove the container: %s", container))
	}()

	if squash, err = execx.String(ctx, "buildah", "commit", "-q", "--squash", "--timestamp", "0", container); err != nil {
		return "", errorsx.Wrap(err, "unable to squash the image")
	}
	squash = strings.TrimSpace(squash)
	defer func() {
		errorsx.Log(errorsx.Wrapf(exec.Command("buildah", "rmi", squash).Run(), "unable to remove the squashed image: %s", squash))
	}()

	cmd = exec.CommandContext(ctx, "buildah", "push", "--compression-format", "gzip", "--force-compression", squash, "oci:"+layout)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err = execx.MaybeRun(cmd); err != nil {
		return "", errorsx.Wrap(err, "unable to write the image's layout")
	}

	if layer, environ, err = squashed(layout); err != nil {
		return "", err
	}

	if root, err = os.CreateTemp(imagedir, "root.*.tar"); err != nil {
		return "", errorsx.Wrap(err, "unable to create the image's archive")
	}
	defer func() { err = errorsx.Compact(err, root.Close(), os.Remove(root.Name())) }()

	if err = withenviron(ctx, layer, environ, root); err != nil {
		return "", err
	}

	if st, err = root.Stat(); err != nil {
		return "", err
	}

	if out, err = os.Create(location); err != nil {
		return "", errorsx.Wrap(err, "unable to create the egiso")
	}
	defer out.Close()

	if toc, err = egiso.Build(io.NewSectionReader(root, 0, st.Size()), out); err != nil {
		return "", errorsx.Wrap(err, "unable to build the egiso")
	}

	if err = os.WriteFile(filepath.Join(imagedir, "toc"), []byte(toc), 0600); err != nil {
		return "", errorsx.Wrap(err, "unable to record the egiso's table of contents")
	}

	// the latest image stays in buildah's storage, so an unchanged rebuild reuses it and keeps its id.
	for stale := range fsx.KeepNewestN(1, fsx.Find(cache, fsx.Levels(1))).Each(ctx) {
		errorsx.Log(errorsx.Wrapf(fsx.RemoveAll(stale), "egiso cache cleanup: %s", stale))
		errorsx.Log(errorsx.Wrapf(exec.Command("buildah", "rmi", filepath.Base(stale)).Run(), "unable to remove the image: %s", filepath.Base(stale)))
	}

	return location + "#" + toc.String(), nil
}

// squashed reads the OCI layout of a squashed image, returning the path of its only layer and the
// environment its configuration starts processes with.
func squashed(layout string) (layer string, environ []string, err error) {
	var (
		index    ocispec.Index
		manifest ocispec.Manifest
		config   ocispec.Image
	)

	blob := func(d digest.Digest) string {
		return filepath.Join(layout, ocispec.ImageBlobsDir, d.Algorithm().String(), d.Encoded())
	}

	if err = decode(filepath.Join(layout, ocispec.ImageIndexFile), &index); err != nil {
		return "", nil, errorsx.Wrap(err, "unable to read the image's index")
	}

	if len(index.Manifests) != 1 {
		return "", nil, errorsx.Errorf("expected one image in the layout, found %d", len(index.Manifests))
	}

	if err = decode(blob(index.Manifests[0].Digest), &manifest); err != nil {
		return "", nil, errorsx.Wrap(err, "unable to read the image's manifest")
	}

	if len(manifest.Layers) != 1 {
		return "", nil, errorsx.Errorf("expected a squashed image with one layer, found %d", len(manifest.Layers))
	}

	if err = decode(blob(manifest.Config.Digest), &config); err != nil {
		return "", nil, errorsx.Wrap(err, "unable to read the image's configuration")
	}

	return blob(manifest.Layers[0].Digest), config.Config.Env, nil
}

// decode reads the JSON document at p into v.
func decode(p string, v any) (err error) {
	var (
		b []byte
	)

	if b, err = os.ReadFile(p); err != nil {
		return err
	}

	return json.Unmarshal(b, v)
}

// withenviron writes the gzipped layer to dst as a tar archive whose etc/environment holds environ,
// since an archive holds only the filesystem and the environment is the image's configuration.
func withenviron(ctx context.Context, layer string, environ []string, dst io.Writer) (err error) {
	var (
		src     *os.File
		gz      *gzip.Reader
		entries iterx.Seq[*tar.Header]
	)

	if src, err = os.Open(layer); err != nil {
		return errorsx.Wrap(err, "unable to open the image's layer")
	}
	defer src.Close()

	if gz, err = gzip.NewReader(src); err != nil {
		return errorsx.Wrap(err, "unable to decompress the image's layer")
	}
	defer gz.Close()

	r := tar.NewReader(gz)
	w := tar.NewWriter(dst)

	entries = tarx.Entries(r)
	for hdr := range entries.Each(ctx) {
		if path.Clean(strings.TrimPrefix(hdr.Name, "./")) == "etc/environment" {
			continue
		}

		if err = tarx.WriteFileToArchive(w, hdr, r); err != nil {
			return errorsx.Wrap(err, "unable to copy the image's layer")
		}
	}

	if err = entries.Err(); err != nil {
		return errorsx.Wrap(err, "unable to read the image's layer")
	}

	environment := strings.Join(append(environ, ""), "\n")
	if err = tarx.WriteFileToArchive(w, &tar.Header{Typeflag: tar.TypeReg, Name: "etc/environment", Mode: 0644, Size: int64(len(environment))}, strings.NewReader(environment)); err != nil {
		return errorsx.Wrap(err, "unable to write the image's environment")
	}

	return w.Close()
}
