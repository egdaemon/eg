package egmd5x

import (
	"hash"

	"github.com/egdaemon/eg/internal/md5x"
)

// digest the provided contents and return the resulting hash.
// if an error occurs during hashing then a nil value is returned.
func Digest[T string | []byte](b T) hash.Hash {
	return md5x.Digest(b)
}

// String to md5 uuid encoded string, shorthand for egdmdx.FormatString(egmd5x.Digest(v))
func String(s string) string {
	return md5x.String(s)
}

// format md5 hash to a uuid encoded string
func FormatString(m hash.Hash) string {
	return md5x.FormatString(m)
}

// format md5 hash to a hex encoded string
func FormatHex(m hash.Hash) string {
	return md5x.FormatHex(m)
}
