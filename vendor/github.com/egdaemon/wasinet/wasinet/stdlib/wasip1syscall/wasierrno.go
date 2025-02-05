package wasip1syscall

import (
	"fmt"
	"syscall"

	"github.com/egdaemon/wasinet/wasinet/ffierrors"
)

const (
	EAGAIN        syscall.Errno = 0x6
	ECANCELED     syscall.Errno = 0xB
	EDOM          syscall.Errno = 0x12
	EINPROGRESS   syscall.Errno = 0x1A
	EINTR         syscall.Errno = 0x1B
	EINVAL        syscall.Errno = 0x1C
	EIO           syscall.Errno = 0x1D
	EISCONN       syscall.Errno = 0x1E
	EMFILE        syscall.Errno = 0x21
	ENOPROTOOPT   syscall.Errno = 0x32
	ENOTCONN      syscall.Errno = 0x35
	ETIMEDOUT     syscall.Errno = 0x49
	EADDRNOTAVAIL syscall.Errno = 0x63
	ECONNREFUSED  syscall.Errno = 0xE
	ENOENT        syscall.Errno = 0x2C
)

var mapped = map[syscall.Errno]syscall.Errno{
	ffierrors.ErrnoSuccess(): ffierrors.ErrnoSuccess(),
	syscall.EINPROGRESS:      EINPROGRESS,
	syscall.ECANCELED:        ECANCELED,
	syscall.EIO:              EIO,
	syscall.EINTR:            EINTR,
	syscall.EISCONN:          EISCONN,
	syscall.ENOTCONN:         ENOTCONN,
	syscall.EAGAIN:           EAGAIN,
	syscall.EINVAL:           EINVAL,
	syscall.ETIMEDOUT:        ETIMEDOUT,
	syscall.EDOM:             EDOM,
	syscall.EMFILE:           EMFILE,
	syscall.ENOPROTOOPT:      ENOPROTOOPT,
	syscall.ECONNREFUSED:     ECONNREFUSED,
	syscall.ENOENT:           ENOENT,
	syscall.EADDRNOTAVAIL:    EADDRNOTAVAIL,
}

// maps native codes to wasi codes.
func ErrnoTranslate(err syscall.Errno) syscall.Errno {
	if errno, ok := mapped[err]; ok {
		return errno
	}

	// unmapped.
	fmt.Printf("unmapped Errno 0x%X - %s\n", int(err), err)
	return err
}
