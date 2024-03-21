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

func PrintFS(d fs.FS) {
	errorsx.MaybeLog(log.Output(2, fmt.Sprintln("--------- FS WALK INITIATED ---------")))
	defer func() { errorsx.MaybeLog(log.Output(3, fmt.Sprintln("--------- FS WALK COMPLETED ---------"))) }()

	err := fs.WalkDir(d, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		errorsx.MaybeLog(log.Output(2, fmt.Sprintln(path)))

		return nil
	})
	if err != nil {
		errorsx.MaybeLog(log.Output(2, fmt.Sprintln("fs walk failed", err)))
	}
}
