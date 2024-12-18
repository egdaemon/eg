package md5x

import (
	"crypto/md5"
	"encoding/hex"
	"hash"

	"github.com/gofrs/uuid"
)

// DigestHex to md5 hex encoded string
func DigestHex(b []byte) string {
	d := md5.Sum(b)
	return hex.EncodeToString(d[:])
}

// String to md5 hex encoded string
func String(s string) string {
	return DigestHex([]byte(s))
}

// DigestX digest byte slice
func DigestX(b []byte) []byte {
	d := md5.Sum(b)
	return d[:]
}

func FormatString(m hash.Hash) string {
	return uuid.FromBytesOrNil(m.Sum(nil)).String()
}
