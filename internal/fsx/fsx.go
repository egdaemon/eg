package fsx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/systemx"
	"github.com/egdaemon/eg/internal/tracex"
)

func Wait(ctx context.Context, d time.Duration, path string) error {
	ctx, done := context.WithTimeout(ctx, d)
	defer done()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			runtime.Gosched()
			continue
		} else if err != nil {
			return err
		} else {
			return nil
		}
	}
}

// Locate - looks for the provided filename up the file tree.
// and returns the path once found, if no path is found then it returns
// the name without a directory, which makes its a relative path.
func Locate(name string) string {
	// fallback to root so it'll stop immediately.
	for dir := systemx.WorkingDirectoryOrDefault("."); dir != "/"; dir = filepath.Dir(dir) {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return name
}

// LocateFirstInDir locates the first file in the given directory by name.
func LocateFirstInDir(dir string, names ...string) (result string) {
	for _, name := range names {
		result = filepath.Join(dir, name)
		if _, err := os.Stat(result); err == nil {
			break
		}
	}

	return result
}

// LocateFirstlocates the first path that exists, or an empty string if none.
func LocateFirst(paths ...string) (result string) {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return result
}

func LocatePhysicalPath(paths ...string) string {
	for _, path := range paths {
		// EvalSymlinks calculates the actual path after evaluating all symlinks
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			continue
		}

		if info, err := os.Stat(realPath); err == nil && info.IsDir() {
			return realPath
		}
	}
	return ""
}

// FileExists returns true IFF a non-directory file exists at the provided path.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		debugx.Println(errorsx.Wrapf(err, "failed to stat path: %s", path))
		return false
	}

	if info.IsDir() {
		return false
	}

	return true
}

// SymlinkExists returns true IFF a file exists and is a symlink to another file at the provided path.
func SymlinkExists(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return errorsx.Wrapf(err, "unable to stat: %s", path)
	}

	// Check if the file's mode indicates it is a symbolic link.
	// os.ModeSymlink is a bitmask for the file type.
	if info.Mode()&os.ModeSymlink == 0 {
		return errorsx.Errorf("not a symlink %v: %s", info.Mode(), path)
	}

	return nil
}

// FileExists returns true IFF a non-directory file exists at the provided path.
func DirExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		debugx.Println(errorsx.Wrapf(err, "failed to stat path: %s", path))
		return err
	}

	if !info.IsDir() {
		return errorsx.Errorf("not a directory %v: %s", info.Mode(), path)
	}

	return nil
}

func PrintFS(d fs.FS) {
	errorsx.Log(log.Output(2, fmt.Sprintln("--------- FS WALK INITIATED ---------")))
	defer func() { errorsx.Log(log.Output(3, fmt.Sprintln("--------- FS WALK COMPLETED ---------"))) }()

	err := fs.WalkDir(d, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info := errorsx.Zero(d.Info())
		errorsx.Log(log.Output(7, fmt.Sprintf("%v %4d %s\n", info.Mode(), info.Size(), path)))

		return nil
	})
	if err != nil {
		errorsx.Log(log.Output(2, fmt.Sprintln("fs walk failed", err)))
	}
}

func PrintDir(d fs.FS) {
	errorsx.Log(log.Output(2, fmt.Sprintln("--------- PRINT DIR INITIATED ---------")))
	defer func() { errorsx.Log(log.Output(3, fmt.Sprintln("--------- PRINT DIR COMPLETED ---------"))) }()

	err := fs.WalkDir(d, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info := errorsx.Zero(d.Info())
		log.Printf("%v %4d %s\n", info.Mode(), info.Size(), info.Name())

		if d.IsDir() && info.Name() != "." {
			return fs.SkipDir
		}

		return nil
	})
	if err != nil {
		errorsx.Log(log.Output(2, fmt.Sprintln("print dir failed", err)))
	}
}

func Clone(ctx context.Context, src string, dst string) (err error) {
	sio, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sio.Close()

	dio, err := os.OpenFile(dst, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer dio.Close()

	if _, err = io.Copy(dio, sio); err != nil {
		return err
	}

	return nil
}

func CloneTree(ctx context.Context, dstdir string, rootdir string, archive fs.FS) (err error) {
	if err = os.MkdirAll(dstdir, 0700); err != nil {
		return err
	}

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
			return nil
		}

		dst := filepath.Join(dstdir, strings.TrimPrefix(path, rootdir))
		if rootdir == path {
			dst = path
		}

		tracex.Println("cloning", rootdir, path, "->", dst, os.FileMode(0755), os.FileMode(0600))

		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}

		c, err := archive.Open(path)
		if err != nil {
			return err
		}
		defer c.Close()

		df, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
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

func MkDirs(perm fs.FileMode, paths ...string) (err error) {
	for _, p := range paths {
		if err = os.MkdirAll(p, perm); err != nil {
			return errorsx.Wrapf(err, "unable to create directory: %s", p)
		}

		if err = os.Chmod(p, perm); err != nil {
			return errorsx.Wrapf(err, "unable to set directory mod: %s", p)
		}
	}

	return nil
}

func ErrIsNotExist(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}
