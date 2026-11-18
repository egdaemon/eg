//go:build linux

package ttyx

import (
	"os"
	"testing"
)

// testpty is a pseudo terminal closed with the test, the test is skipped where there is none.
func testpty(t *testing.T) (parent, child *os.File) {
	t.Helper()
	parent, child, err := openpty()
	if err != nil {
		t.Skip("no pseudo terminal available", err)
	}
	t.Cleanup(func() {
		child.Close()
		parent.Close()
	})
	return parent, child
}
