//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package ttyx

import "os"

func devkey(f *os.File) (string, error) {
	return "name:" + f.Name(), nil
}

// cooked has no output processing to restore on these platforms.
func cooked(fd int, keepsig bool) error {
	return nil
}
