package egfs

import (
	"context"
	"io"
	"io/fs"
	"iter"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
)

func FindFirst(tree fs.FS, pattern string) string {
	for p := range Find(tree, pattern) {
		return p
	}

	return ""
}

func Find(tree fs.FS, pattern string) iter.Seq[string] {
	return func(yield func(string) bool) {
		err := fs.WalkDir(tree, ".", func(current string, d fs.DirEntry, err error) error {
			if err != nil {
				return errorsx.Wrapf(err, "erroring walking tree: %s", current)
			}

			if ok, err := path.Match(pattern, d.Name()); err != nil {
				return err
			} else if !ok {
				return nil
			}

			if !yield(current) {
				return fs.SkipAll
			}

			return nil
		})

		errorsx.Log(errorsx.Wrap(err, "unable to find modules"))
	}
}

// String returns the contents of the file at the provided path.
func String(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// StringLazy returns a function that reads the file at the provided path when called.
func StringLazy(path string) func() (string, error) {
	return func() (string, error) {
		return String(path)
	}
}

// FileExists returns true IFF a non-directory file exists at the provided path.
func FileExists(path string) bool {
	info, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	}

	if info.IsDir() {
		return false
	}

	return true
}

// FileExistsFn returns a function compatible with eg.WhenFn that checks
// if a non-directory file exists at the provided path.
func FileExistsFn(path string) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		return FileExists(path)
	}
}

// FileNotExistsFn returns a function compatible with eg.WhenFn that checks
// that no non-directory file exists at the provided path.
func FileNotExistsFn(path string) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		return !FileExists(path)
	}
}

// DirExists returns true IFF a non-directory file exists at the provided path.
func DirExists(path string) bool {
	info, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	}

	return info.IsDir()
}

// DirExistsFn returns a function compatible with eg.WhenFn that checks
// if a directory exists at the provided path.
func DirExistsFn(path string) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		return DirExists(path)
	}
}

// DirNotExistsFn returns a function compatible with eg.WhenFn that checks
// that no directory exists at the provided path.
func DirNotExistsFn(path string) func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		return !DirExists(path)
	}
}

func MkDirs(perm fs.FileMode, paths ...string) (err error) {
	for _, p := range paths {
		if err = os.MkdirAll(p, perm); err != nil {
			return errorsx.Wrapf(err, "unable to create directory: %s", p)
		}
	}

	return nil
}

// print the list of files an directories contained within the FS.
func Inspect(ctx context.Context, archive fs.FS) (err error) {
	return fs.WalkDir(archive, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// allow clone tree to be cancellable.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Println(path, "->", info.Mode().Perm())

		return nil
	})
}

func CloneFS(ctx context.Context, dstdir string, rootdir string, archive fs.FS) (err error) {
	return fs.WalkDir(archive, rootdir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// allow clone tree to be cancellable.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() && rootdir == path {
			info, err := d.Info()
			if err != nil {
				return err
			}

			return os.MkdirAll(dstdir, info.Mode().Perm())
		}

		rel := strings.TrimPrefix(path, rootdir)
		if rootdir == path {
			rel = path
		}

		dst := filepath.Join(dstdir, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		log.Println("cloning", rootdir, path, "->", dst, info.Mode().Perm())

		if d.IsDir() {
			return os.MkdirAll(dst, info.Mode().Perm())
		}

		if !d.IsDir() && rootdir == path {
			if err = os.MkdirAll(filepath.Dir(dst), 0777); err != nil {
				return err
			}
		}

		c, err := archive.Open(path)
		if err != nil {
			return err
		}
		defer c.Close()

		df, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		defer df.Close()

		if _, err := io.Copy(df, c); err != nil {
			return err
		}

		return nil
	})
}

type installoptions struct {
	perm fs.FileMode
}

// InstallOption modifies how Install writes the file.
type InstallOption func(*installoptions)

// Perm sets the permissions of the installed file, defaults to 0644.
func Perm(perm fs.FileMode) InstallOption {
	return func(o *installoptions) {
		o.perm = perm
	}
}

// Install writes the contents of src to dst, creating parent directories as needed
// and truncating any existing file. src is consumed by the first run of the operation.
func Install(dst string, src io.Reader, options ...InstallOption) eg.OpFn {
	cfg := installoptions{perm: 0644}
	for _, opt := range options {
		opt(&cfg)
	}

	return func(ctx context.Context, o eg.Op) (err error) {
		if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return errorsx.Wrapf(err, "unable to create directory for: %s", dst)
		}

		df, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, cfg.perm)
		if err != nil {
			return errorsx.Wrapf(err, "unable to open: %s", dst)
		}
		defer func() {
			if cerr := df.Close(); err == nil {
				err = cerr
			}
		}()

		// OpenFile only applies perm when creating and is masked by umask.
		if err = df.Chmod(cfg.perm); err != nil {
			return errorsx.Wrapf(err, "unable to set permissions: %s", dst)
		}

		if _, err = io.Copy(df, src); err != nil {
			return errorsx.Wrapf(err, "unable to write: %s", dst)
		}

		return nil
	}
}
