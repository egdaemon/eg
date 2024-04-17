package fsx

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

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
