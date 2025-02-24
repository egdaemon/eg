package md5x

import (
	"crypto/md5"
	"encoding/hex"
	"hash"

	"github.com/gofrs/uuid"
)

// digest the provided contents and return the resulting hash.
// if an error occurs during hashing then a nil value is returned.
func Digest[T string | []byte](b T) hash.Hash {
	v := md5.New()
	y := []byte(b)
	if n, err := v.Write(y); err != nil || n < len(y) {
		return nil
	}

	return v
}

// String to md5 uuid encoded string
func String(s string) string {
	return FormatString(Digest(s))
}

func FormatString(m hash.Hash) string {
	return uuid.FromBytesOrNil(m.Sum(nil)).String()
}

func FormatHex(m hash.Hash) string {
	return hex.EncodeToString(m.Sum(nil))
}
