package md5x

import (
	"crypto/md5"
	"encoding/hex"
	"hash"

	"github.com/gofrs/uuid"
)

// Digest to md5 hex encoded string
func Digest(b []byte) string {
	d := md5.Sum(b)
	return hex.EncodeToString(d[:])
}

// DigestString to md5 hex encoded string
func DigestString(s string) string {
	return Digest([]byte(s))
}

// DigestX digest byte slice
func DigestX(b []byte) []byte {
	d := md5.Sum(b)
	return d[:]
}

func FormatString(m hash.Hash) string {
	return uuid.FromBytesOrNil(m.Sum(nil)).String()
}
