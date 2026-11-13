// Package ffiegenv provides the guest side of the egenv functions the host implements.
package ffiegenv

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffiguest"
)

// separator must match the host, see interp/runtime/wasi/ffiegenv.Separator.
const separator = "\x00"

// Environ returns the environ the runner provided to the workload, as stored in its environment file.
func Environ(ctx context.Context) ([]string, error) {
	var (
		length uint32
		buf    = make([]byte, 4096)
	)

	for {
		bufptr, bufcap := ffiguest.Bytes(buf)
		errcode := environ(ffiguest.ContextDeadline(ctx), bufptr, bufcap, unsafe.Pointer(&length))

		err := ffiguest.Error(errcode, fmt.Errorf("environ failed"))
		if errcode == ffierrors.ErrBufferTooSmall && length > uint32(len(buf)) {
			buf = make([]byte, length)
			continue
		}

		if err != nil {
			return nil, err
		}

		if length > uint32(len(buf)) {
			return nil, errors.New("host reported an environ larger than the buffer")
		}

		if length == 0 {
			return nil, nil
		}

		return strings.Split(string(buf[:length]), separator), nil
	}
}
