package bytesx

import (
	"fmt"

	"github.com/egdaemon/eg/internal/md5x"
)

type Unit int64

func (t Unit) Format(f fmt.State, verb rune) {
	div := int64(1)
	suffix := ""
	switch {
	case t > EiB:
		div = EiB
		suffix = "e"
	case t > PiB:
		div = PiB
		suffix = "p"
	case t > TiB:
		div = TiB
		suffix = "t"
	case t > GiB:
		div = GiB
		suffix = "g"
	case t > MiB:
		div = MiB
		suffix = "m"
	case t > KiB:
		div = KiB
		suffix = "k"
	}

	f.Write([]byte(fmt.Sprintf("%d%s", uint64(float64(t)/float64(div)), suffix)))
}

// base 2 byte units
const (
	_   Unit = iota
	KiB      = 1 << (10 * iota)
	MiB
	GiB
	TiB
	PiB
	EiB
)

type Debug []byte

func (t Debug) Format(f fmt.State, verb rune) {
	switch verb {
	case 's':
		_, _ = f.Write([]byte(md5x.DigestHex(t)))
	case 'v':
		_, _ = f.Write([]byte(md5x.DigestHex(t)))
		if f.Flag('+') {
			_, _ = f.Write([]byte(" "))
			_, _ = f.Write([]byte(fmt.Sprintf("%v", []byte(t))))
		}

	}
}
