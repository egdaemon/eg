package fsx

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
)

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

// FileExists returns true IFF a non-directory file exists at the provided path.
func DirExists(path string) bool {
	info, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	}

	return info.IsDir()
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

func CloneTree(ctx context.Context, dstdir string, rootdir string, archive fs.FS) (err error) {
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

		log.Println("cloning", rootdir, path, "->", dst, os.FileMode(0755), os.FileMode(0600))

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
	}

	return nil
}
