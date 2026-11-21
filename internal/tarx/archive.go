package tarx

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/cryptox"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/uuidx"
)

// Entries yields the headers of the archive tr reads, with tr positioned at the contents of each
// entry while it is yielded.
func Entries(tr *tar.Reader) iterx.Seq[*tar.Header] {
	return iterx.New(func(ctx context.Context, yield func(*tar.Header) bool) (err error) {
		var (
			header *tar.Header
		)

		for header, err = tr.Next(); err == nil; header, err = tr.Next() {
			if !yield(header) {
				return nil
			}
		}

		return iox.IgnoreEOF(err)
	})
}

// WriteFileToArchive write a file to a tar given the contents and a header.
func WriteFileToArchive(tw *tar.Writer, hdr *tar.Header, reader io.Reader) (err error) {
	if err = tw.WriteHeader(hdr); err != nil {
		return errorsx.Wrap(err, "failed to write header for tar archive")
	}

	if _, err = io.Copy(tw, reader); err != nil {
		return errorsx.Wrap(err, "failed to copy content")
	}

	return nil
}

// Packer writes a prepared entry to the archive.
type Packer func(tw *tar.Writer) error

// Paths yields a Packer for every file and directory beneath each of the paths, named relative to the path
// it was found in. the path itself is not an entry when it is a directory.
func Paths(paths ...string) iterx.Seq[Packer] {
	return iterx.New(func(ctx context.Context, yield func(Packer) bool) error {
		for _, basepath := range paths {
			walker := func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// skip the root directory itself.
				if basepath == path && info.IsDir() {
					return nil
				}

				if !yield(func(tw *tar.Writer) error { return write(basepath, path, tw, info) }) {
					return iterx.ErrIterationComplete
				}

				return nil
			}

			if err := filepath.Walk(basepath, walker); errors.Is(err, iterx.ErrIterationComplete) {
				return nil
			} else if err != nil {
				return err
			}
		}

		return nil
	})
}

// File yields a Packer for the file at src, which is named dst within the archive.
func File(src, dst string) iterx.Seq[Packer] {
	return file(src, dst, false)
}

// MaybeFile is File that yields nothing when src does not exist, including when src is blank.
func MaybeFile(src, dst string) iterx.Seq[Packer] {
	return file(src, dst, true)
}

func file(src, dst string, optional bool) iterx.Seq[Packer] {
	return iterx.New(func(ctx context.Context, yield func(Packer) bool) error {
		info, err := os.Stat(src)
		if optional && errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		if err != nil {
			return errorsx.Wrapf(err, "failed to stat path: %s", src)
		}

		yield(func(tw *tar.Writer) (err error) {
			var (
				f      *os.File
				header *tar.Header
			)

			if f, err = os.Open(src); err != nil {
				return errorsx.Wrap(err, "failed to open path")
			}
			defer f.Close()

			if header, err = tar.FileInfoHeader(info, ""); err != nil {
				return errorsx.Wrap(err, "failed to created header")
			}
			header.Name = dst

			return WriteFileToArchive(tw, header, f)
		})

		return nil
	})
}

// Pack the contents into the archive. caller is responsible for rewinding the writer.
func Pack(ctx context.Context, dst io.Writer, contents ...iterx.Seq[Packer]) (err error) {
	var (
		gw *gzip.Writer
		tw *tar.Writer
	)

	if s, ok := dst.(io.Seeker); ok {
		defer errorsx.Log(errorsx.Wrap(iox.Rewind(s), "unable to rewind archive"))
	}

	gw = gzip.NewWriter(dst)
	defer gw.Close()
	tw = tar.NewWriter(gw)
	defer tw.Close()

	for _, c := range contents {
		for packer := range c.Each(ctx) {
			if err = packer(tw); err != nil {
				return err
			}
		}

		if err = c.Err(); err != nil {
			return err
		}
	}

	return errorsx.Wrap(tw.Flush(), "failed to flush archive")
}

// Unpack the archive from the reader into the root directory.
func Unpack(ctx context.Context, root string, r io.Reader) (err error) {
	var (
		dst     *os.File
		gzr     *gzip.Reader
		tr      *tar.Reader
		entries iterx.Seq[*tar.Header]
	)

	if err = os.MkdirAll(root, 0700); err != nil {
		return errorsx.Wrap(err, "unable to ensure root directory")
	}
	if gzr, err = gzip.NewReader(r); err != nil {
		return errorsx.Wrap(err, "failed to create gzip reader")
	}
	defer gzr.Close()

	tr = tar.NewReader(gzr)
	entries = Entries(tr)

	for header := range entries.Each(ctx) {
		// the target location where the dir/file should be created
		target := filepath.Join(root, filepath.Clean(header.Name))

		// check the file type
		switch header.Typeflag {
		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err = os.Stat(target); os.IsNotExist(err) {
				if err = os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
					return errorsx.Wrapf(err, "failed to create directory: %s", target)
				}
			} else if err != nil {
				return errorsx.Wrapf(err, "failed to stat directory: %s", target)
			}
		// if it's a file create it
		case tar.TypeReg:
			writefile := func() error {
				// log.Println("writing", header.Name, "->", target)
				if dst, err = os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)); err != nil {
					return errorsx.Wrapf(err, "failed to open file: %s", target)
				}
				defer dst.Close()

				// copy over contents
				if _, err = io.Copy(dst, tr); err != nil {
					return errorsx.Wrapf(err, "failed to copy contents: %s", target)
				}

				return nil
			}

			if err = writefile(); err != nil {
				return err
			}
		}
	}

	return entries.Err()
}

// prints to stderr information about the archive
func Inspect(ctx context.Context, r io.Reader) (err error) {
	var (
		gzr     *gzip.Reader
		entries iterx.Seq[*tar.Header]
	)

	if s, ok := r.(io.Seeker); ok {
		if err = iox.Rewind(s); err != nil {
			return errorsx.Wrap(err, "unable to seek to start of file")
		}

		defer func() { errorsx.Log(errorsx.Wrap(iox.Rewind(s), "unable to rewind")) }()
	}

	if gzr, err = gzip.NewReader(r); err != nil {
		return errorsx.Wrap(err, "failed to create gzip reader")
	}
	defer gzr.Close()

	entries = Entries(tar.NewReader(gzr))

	for header := range entries.Each(ctx) {
		if header.Typeflag == tar.TypeReg {
			log.Println(header.Name, header.Size, header.FileInfo().Size())
		}
	}

	return entries.Err()
}

func write(basepath, path string, tw *tar.Writer, info os.FileInfo) (err error) {
	var (
		src    *os.File
		header *tar.Header
		target string
	)

	if target, err = filepath.Rel(basepath, path); err != nil {
		return errorsx.Wrapf(err, "failed to compute path: %s", path)
	}

	// base and path are identical
	if target == "." {
		target = filepath.Base(path)
	}

	// log.Println("writing", path, "->", target)
	if src, err = os.Open(path); err != nil {
		return errorsx.Wrap(err, "failed to open path")
	}
	defer src.Close()

	if header, err = tar.FileInfoHeader(info, path); err != nil {
		return errorsx.Wrap(err, "failed to created header")
	}
	header.Name = target

	if err = tw.WriteHeader(header); err != nil {
		return errorsx.Wrapf(err, "failed to write header to tar archive: %s", path)
	}

	// return on directories since there will be no content to tar
	if info.Mode().IsDir() {
		return nil
	}

	if _, err = io.Copy(tw, src); err != nil {
		return errorsx.Wrapf(err, "failed to write contexts to tar archive: %s", path)
	}

	return nil
}

type randomized struct {
	min  uint32
	max  uint32
	n    uint8
	seed []byte
}

type option func(*randomized)
type options []option

func Randomized() options {
	return options(nil)
}

func (opts options) Min(min uint32) options {
	return append(opts, func(r *randomized) {
		r.min = min
	})
}

func (opts options) Max(max uint32) options {
	return append(opts, func(r *randomized) {
		r.max = max
	})
}

func (opts options) N(n uint8) options {
	return append(opts, func(r *randomized) {
		r.n = n
	})
}

func (opts options) New(tw *tar.Writer) error {
	r := langx.Clone(randomized{
		min:  bytesx.KiB,
		max:  5 * bytesx.KiB,
		n:    3,
		seed: uuidx.NewV4().Bytes(),
	}, opts...)

	src := cryptox.NewChaCha8(r.seed)
	for range r.n {
		if _, err := io.Copy(tw, io.LimitReader(src, int64(r.min+uint32(rand.IntN(int(r.max-r.min)))))); err != nil {
			return err
		}
	}

	return nil
}

// Size sums the sizes recorded in the archive's headers, i.e.) the uncompressed size of its contents.
func Size(ctx context.Context, r io.Reader) (n int64, err error) {
	var (
		gzr     *gzip.Reader
		entries iterx.Seq[*tar.Header]
	)

	if gzr, err = gzip.NewReader(r); err != nil {
		return 0, errorsx.Wrap(err, "failed to create gzip reader")
	}
	defer gzr.Close()

	entries = Entries(tar.NewReader(gzr))

	for header := range entries.Each(ctx) {
		n += header.Size
	}

	return n, errorsx.Wrap(entries.Err(), "unable to read archive header")
}
