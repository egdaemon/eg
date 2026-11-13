// Package ffiegenv provides the host side of the egenv functions that need the host to read on behalf of the guest.
package ffiegenv

import (
	"context"
	"log"
	"strings"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffi"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffierrors"
	"github.com/tetratelabs/wazero/api"
)

// Separator joins the entries of an environ written to the guest. environment variables cannot contain NUL.
const Separator = "\x00"

// Environ writes the environ stored in the file at path to the guest. the runner writes the file 0600 so the
// unprivileged user commands run as cannot read it, the host reads it instead.
//
// the entries are joined with Separator into the buffer at bufptr and the length of the result is written to
// lenptr. when the buffer is smaller than the result nothing is written to it and ErrBufferTooSmall is returned,
// the guest retries with a buffer of the length written to lenptr.
func Environ(path string) func(
	ctx context.Context,
	m api.Module,
	deadline int64, // context.Context
	bufptr uint32, bufcap uint32, // return []byte
	lenptr uint32, // return uint32
) (errcode uint32) {
	return func(ctx context.Context, m api.Module, deadline int64, bufptr, bufcap, lenptr uint32) (errcode uint32) {
		_, done := ffi.ReadMicroDeadline(ctx, deadline)
		defer done()

		environ, err := envx.Build().FromPath(path).Environ()
		if err != nil {
			log.Println("unable to read environ", path, err)
			return ffierrors.ErrUnrecoverable
		}

		encoded := strings.Join(environ, Separator)

		if !m.Memory().WriteUint32Le(lenptr, uint32(len(encoded))) {
			log.Println("unable to write environ length")
			return ffierrors.ErrUnrecoverable
		}

		if uint32(len(encoded)) > bufcap {
			return ffierrors.ErrBufferTooSmall
		}

		if !m.Memory().WriteString(bufptr, encoded) {
			log.Println("unable to write environ")
			return ffierrors.ErrUnrecoverable
		}

		return 0
	}
}
