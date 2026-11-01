package wasinet

import (
	"github.com/egdaemon/wasinet/wasinet/stdlib/wasip1syscall"
)

type sockaddr interface {
	Sockaddr() wasip1syscall.RawSocketAddress
}
