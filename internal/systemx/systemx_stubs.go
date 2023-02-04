//go:build !linux
// +build !linux

package systemx

import (
	"errors"
	"os"
	"time"
)

func FileCreatedAt(info os.FileInfo) (ctime time.Time, err error) {
	return ctime, errors.New("unable to retrieve creation time of file outside of linux")
}
