//go:build !linux && !darwin

package ttyx

import (
	"errors"
	"os"
)

func openpty() (parent, child *os.File, err error) {
	return nil, nil, errors.ErrUnsupported
}

func resize(ctl, parent *os.File) error {
	return errors.ErrUnsupported
}

func setsize(parent *os.File, rows, cols uint16) error {
	return errors.ErrUnsupported
}
