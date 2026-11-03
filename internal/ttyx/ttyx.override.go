package ttyx

import "os"

// Override replaces what is considered stdin and the path of the controlling terminal, for tests
// of commands outside this package that need to pose as piped or terminal input regardless of the
// machine running them. the returned function restores the previous values.
func Override(in func() *os.File, path string) (restore func()) {
	pin, ppath := stdin, ttypath
	stdin, ttypath = in, path

	return func() {
		stdin, ttypath = pin, ppath
	}
}
