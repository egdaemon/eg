package bytesx

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"github.com/dustin/go-humanize"
)

type IEC600272 uint64

// Format according to the IEC600272 standard.
func (t IEC600272) Format(f fmt.State, verb rune) {
	var div uint64 = 1
	suffix := ""

	// Determine the appropriate prefix and divisor based on the IEC standards
	switch {
	case t >= EiB:
		div = EiB
		suffix = "EiB"
	case t >= PiB:
		div = PiB
		suffix = "PiB"
	case t >= TiB:
		div = TiB
		suffix = "TiB"
	case t >= GiB:
		div = GiB
		suffix = "GiB"
	case t >= MiB:
		div = MiB
		suffix = "MiB"
	case t >= KiB:
		div = KiB
		suffix = "KiB"
	}

	value := math.Round(float64(t) / float64(div))

	f.Write([]byte(fmt.Sprintf("%d%s", uint64(value), suffix)))
}

type FormatBinary = IEC600272
type FormatSI uint64

// Format according to the SI standards of measurement.
func (t FormatSI) Format(f fmt.State, verb rune) {
	var div float64 = 1.0 // Use float64 for decimal division
	suffix := ""          // Default suffix for bytes

	// Define SI decimal powers (powers of 1000) as local constants.
	// These are used for comparison and division in this function only.
	const (
		KB_SI float64 = 1000
		MB_SI float64 = 1000 * KB_SI
		GB_SI float64 = 1000 * MB_SI
		TB_SI float64 = 1000 * GB_SI
		PB_SI float64 = 1000 * TB_SI
		EB_SI float64 = 1000 * PB_SI
	)

	switch {
	case float64(t) >= EB_SI:
		div = EB_SI
		suffix = "EB"
	case float64(t) >= PB_SI:
		div = PB_SI
		suffix = "PB"
	case float64(t) >= TB_SI:
		div = TB_SI
		suffix = "TB"
	case float64(t) >= GB_SI:
		div = GB_SI
		suffix = "GB"
	case float64(t) >= MB_SI:
		div = MB_SI
		suffix = "MB"
	case float64(t) >= KB_SI:
		div = KB_SI
		suffix = "kB" // Kilobyte uses lowercase 'k' as per SI standard
	}

	value := math.Round(float64(t) / float64(div))

	f.Write([]byte(fmt.Sprintf("%d%s", uint64(value), suffix)))
}

type Unit uint64

func (t Unit) MarshalText() (text []byte, err error) {
	return []byte(humanize.IBytes(uint64(t))), nil
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

	switch verb {
	case 'X':
		suffix = strings.ToUpper(suffix)
	default:
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
