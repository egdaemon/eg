//go:build !linux

package egiso

import (
	"io/fs"
)

// Lstat describes name without following a final symlink.
func (t hostdir) Lstat(name string) (fs.FileInfo, error) {
	return t.Root.Lstat(name)
}
