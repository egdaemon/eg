package bytesx

import "bytes"

// base 2 byte units
const (
	_   = iota
	KiB = 1 << (10 * iota)
	MiB
	GiB
	TiB
	PiB
	EiB
)

// NewSizedBuffer return a buffer of n bytes
func NewSizedBuffer(n int) *bytes.Buffer {
	return bytes.NewBuffer(make([]byte, 0, n))
}
