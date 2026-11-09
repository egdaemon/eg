package egvm

import (
	"fmt"
	"io"
	"io/fs"
)

// File is an entry of an initramfs: a directory when Mode is one, otherwise a regular file
// holding Data.
type File struct {
	Name string
	Mode fs.FileMode
	Data []byte
}

// Initramfs writes files as a newc cpio archive, the format the kernel unpacks into its initial
// root filesystem.
func Initramfs(w io.Writer, files ...File) error {
	for i, f := range files {
		mode := uint32(0o100000) | uint32(f.Mode.Perm())
		if f.Mode.IsDir() {
			mode = 0o040000 | uint32(f.Mode.Perm())
		}
		if err := entry(w, uint32(i+1), mode, f.Name, f.Data); err != nil {
			return err
		}
	}
	return entry(w, 0, 0, "TRAILER!!!", nil)
}

func entry(w io.Writer, ino uint32, mode uint32, name string, data []byte) error {
	header := fmt.Sprintf("070701%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X%08X",
		ino, mode, 0, 0, 1, 0, len(data), 0, 0, 0, 0, len(name)+1, 0)
	if _, err := io.WriteString(w, header+name+"\x00"+pad(len(header)+len(name)+1)); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err := io.WriteString(w, pad(len(data)))
	return err
}

// pad returns the zeros that align n bytes to the archive's four byte boundary.
func pad(n int) string {
	return string(make([]byte, (4-n%4)%4))
}
