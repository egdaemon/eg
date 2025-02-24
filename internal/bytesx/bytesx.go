package bytesx

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"github.com/dustin/go-humanize"
)

type Unit uint64

func (t Unit) MarshalText() (text []byte, err error) {
	return []byte(humanize.Bytes(uint64(t))), nil
}

func (t *Unit) UnmarshalText(text []byte) (err error) {
	tmp, err := humanize.ParseBytes(string(text))
	*t = Unit(tmp)
	return err
}

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
	digest := func(b []byte) string {
		d := md5.Sum(b)
		return hex.EncodeToString(d[:])
	}
	switch verb {
	case 's':
		_, _ = f.Write([]byte(digest(t)))
	case 'v':
		_, _ = f.Write([]byte(digest(t)))
		if f.Flag('+') {
			_, _ = f.Write([]byte(" "))
			_, _ = f.Write([]byte(fmt.Sprintf("%v", []byte(t))))
		}

	}
}
