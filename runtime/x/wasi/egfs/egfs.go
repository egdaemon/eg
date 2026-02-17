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

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
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

		debugx.Println("cloning", rootdir, path, "->", dst, info.Mode().Perm())

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
