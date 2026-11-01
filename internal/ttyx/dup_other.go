//go:build !unix

package ttyx

import (
	"errors"
	"os"
)

func dup(f *os.File) (*os.File, error) {
	return nil, errors.ErrUnsupported
}
