package fsx

import (
	"os"
	"path/filepath"
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
