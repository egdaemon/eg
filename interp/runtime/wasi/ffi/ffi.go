package ffi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"syscall"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/tetratelabs/wazero/api"
)

type Memory interface {

	// // ReadUint32Le reads a uint32 in little-endian encoding from the underlying buffer at the offset in or returns
	// // false if out of range.
	ReadUint32Le(offset uint32) (uint32, bool)

	Read(offset, byteCount uint32) ([]byte, bool)

	// WriteUint32Le writes the value in little-endian encoding to the underlying buffer at the offset in or returns
	// false if out of range.
	WriteUint32Le(offset, v uint32) bool

	// // Write writes the slice to the underlying buffer at the offset or returns false if out of range.
	Write(offset uint32, v []byte) bool
}

func ReadString(m api.Memory, offset uint32, len uint32) (string, error) {
	var (
		ok   bool
		data []byte
	)

	if data, ok = m.Read(offset, len); !ok {
		return "", errors.New("unable to read string")
	}

	return string(data), nil
}

func ReadStringArray(m api.Memory, offset uint32, length uint32, argssize uint32) (args []string, err error) {
	args = make([]string, 0, length)

	for offset, i := offset, uint32(0); i < length*2; offset, i = offset+(2*argssize), i+2 {
		var (
			data []byte
		)

		if data, err = ReadArrayElement(m, offset, argssize); err != nil {
			return nil, err
		}

		args = append(args, string(data))
	}

	return args, nil
}

func ReadArrayElement(m api.Memory, offset, len uint32) (data []byte, err error) {
	var (
		ok            bool
		eoffset, elen uint32
	)

	if eoffset, ok = m.ReadUint32Le(offset); !ok {
		return nil, errors.New("unable to read element offset")
	}

	if elen, ok = m.ReadUint32Le(offset + len); !ok {
		return nil, errors.New("unable to read element byte length")
	}

	if data, ok = m.Read(eoffset, elen); !ok {
		return nil, errors.New("unable to read element bytes")
	}

	return data, nil
}

func ReadMicroDeadline(ctx context.Context, deadline int64) (context.Context, context.CancelFunc) {
	return context.WithDeadline(ctx, time.UnixMicro(deadline))
}

func NewFile(m api.Memory, root fs.FS, fd int64, offset uint32, l uint32) (_ fs.File, err error) {
	var (
		name string
	)

	if name, err = ReadString(m, offset, l); err != nil {
		return nil, err
	}

	return root.Open(name)
}

func ReadBytes(m api.Memory, offset uint32, len uint32) (data []byte, err error) {
	var (
		ok bool
	)

	if data, ok = m.Read(offset, len); !ok {
		return nil, errors.New("unable to read string")
	}

	return data, nil
}

func ReadJSON(m api.Memory, offset uint32, len uint32, v interface{}) (err error) {
	var (
		ok      bool
		encoded []byte
	)

	if encoded, ok = m.Read(offset, len); !ok {
		return fmt.Errorf("unable to read json encoded data from memory: %d, %d", offset, len)
	}

	if err = json.Unmarshal(encoded, &v); err != nil {
		return errorsx.Wrap(err, "unable to deserialize json")
	}

	return nil
}

func ErrnoSuccess() syscall.Errno {
	return syscall.Errno(0x0)
}

func Errno(err error) syscall.Errno {
	if err == nil {
		return ErrnoSuccess()
	}

	if err == syscall.EAGAIN {
		return syscall.EAGAIN
	}

	return makeErrnoSlow(err)
}

func makeErrnoSlow(err error) (ret syscall.Errno) {
	var timeout interface{ Timeout() bool }
	if errors.As(err, &ret) {
		return ret
	}
	switch {
	case errors.Is(err, context.Canceled):
		return syscall.ECANCELED
	case errors.Is(err, context.DeadlineExceeded):
		return syscall.ETIMEDOUT
	case errors.Is(err, io.ErrUnexpectedEOF),
		errors.Is(err, fs.ErrClosed),
		errors.Is(err, net.ErrClosed):
		return syscall.EIO
	}

	if errors.As(err, &timeout) {
		if timeout.Timeout() {
			return syscall.ETIMEDOUT
		}
	}

	panic(fmt.Errorf("unexpected error: %v", err))
}
