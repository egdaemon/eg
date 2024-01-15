package tarx

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/internal/iox"
	"github.com/pkg/errors"
)

// Pack the set of paths into the archive. caller is responsible for rewinding the writer.
func Pack(dst io.Writer, paths ...string) (err error) {
	var (
		gw *gzip.Writer
		tw *tar.Writer
	)

	if s, ok := dst.(io.Seeker); ok {
		defer errorsx.MaybeLog(errorsx.Wrap(iox.Rewind(s), "unable to rewind archive"))
	}

	gw = gzip.NewWriter(dst)
	defer gw.Close()
	tw = tar.NewWriter(gw)
	defer tw.Close()

	for _, basepath := range paths {
		walker := func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// skip the root directory itself.
			if basepath == path && info.IsDir() {
				return nil
			}

			return write(basepath, path, tw, info)
		}

		if err = filepath.Walk(basepath, walker); err != nil {
			return err
		}
	}

	return errors.Wrap(tw.Flush(), "failed to flush archive")
}

// Unpack the archive from the reader into the root directory.
func Unpack(root string, r io.Reader) (err error) {
	var (
		dst *os.File
		gzr *gzip.Reader
		tr  *tar.Reader
	)

	if err = os.MkdirAll(root, 0700); err != nil {
		return errors.Wrap(err, "unable to ensure root directory")
	}
	if gzr, err = gzip.NewReader(r); err != nil {
		return errors.Wrap(err, "failed to create gzip reader")
	}
	defer gzr.Close()

	tr = tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		switch {
		// if no more files are found return
		case err == io.EOF:
			return nil
		// return any other error
		case err != nil:
			return err
		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(root, header.Name)

		// check the file type
		switch header.Typeflag {
		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err = os.Stat(target); os.IsNotExist(err) {
				if err = os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
					return errors.Wrapf(err, "failed to create directory: %s", target)
				}
			} else if err != nil {
				return errors.Wrapf(err, "failed to stat directory: %s", target)
			}
		// if it's a file create it
		case tar.TypeReg:
			writefile := func() error {
				if dst, err = os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)); err != nil {
					return errors.Wrapf(err, "failed to open file: %s", target)
				}
				defer dst.Close()

				// copy over contents
				if _, err = io.Copy(dst, tr); err != nil {
					return errors.Wrapf(err, "failed to copy contents: %s", target)
				}

				return nil
			}

			if err := writefile(); err != nil {
				return err
			}
		}
	}
}

// prints to stderr information about the archive
func Inspect(r io.Reader) (err error) {
	var (
		gzr *gzip.Reader
		tr  *tar.Reader
	)

	if s, ok := r.(io.Seeker); ok {
		if err = iox.Rewind(s); err != nil {
			return errors.Wrap(err, "unable to seek to start of file")
		}
	}

	if gzr, err = gzip.NewReader(r); err != nil {
		return errors.Wrap(err, "failed to create gzip reader")
	}
	defer gzr.Close()

	tr = tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		switch {
		// if no more files are found return
		case err == io.EOF:
			return nil
		// return any other error
		case err != nil:
			return err
		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		switch header.Typeflag {
		// if its a dir ignore
		case tar.TypeDir:
		// if it's a file create it
		case tar.TypeReg:
			log.Println(header.Name, header.Size, header.FileInfo().Size())
		}
	}
}

func write(basepath, path string, tw *tar.Writer, info os.FileInfo) (err error) {
	var (
		src    *os.File
		header *tar.Header
		target string
	)

	if target, err = filepath.Rel(basepath, path); err != nil {
		return errors.Wrapf(err, "failed to compute path: %s", path)
	}

	if src, err = os.Open(path); err != nil {
		return errors.Wrap(err, "failed to open path")
	}
	defer src.Close()

	if header, err = tar.FileInfoHeader(info, path); err != nil {
		return errors.Wrap(err, "failed to created header")
	}
	header.Name = target

	if err = tw.WriteHeader(header); err != nil {
		return errors.Wrapf(err, "failed to write header to tar archive: %s", path)
	}

	// return on directories since there will be no content to tar
	if info.Mode().IsDir() {
		return nil
	}

	if _, err = io.Copy(tw, src); err != nil {
		return errors.Wrapf(err, "failed to write contexts to tar archive: %s", path)
	}

	return nil
}
