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
