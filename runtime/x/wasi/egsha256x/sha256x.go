package egsha256x

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/debugx"
)

// digest the provided contents and return the resulting hash.
// if an error occurs during hashing then a nil value is returned.
func Digest[T string | []byte](b T) hash.Hash {
	v := sha256.New()
	y := []byte(b)
	if n, err := v.Write(y); err != nil || n < len(y) {
		return nil
	}

	return v
}

// digest the provided reader and return the resulting hash.
// if an error occurs during hashing then a nil value is returned.
func DigestIO(r io.Reader) hash.Hash {
	var buf [16 * 1024]byte

	v := sha256.New()
	if _, err := io.CopyBuffer(v, r, buf[:]); err != nil {
		debugx.Println("failed to digest from reader", err)
		return nil
	}

	return v
}

// digest the contents from the provided filename and return the resulting hash.
// if an error occurs during hashing then a nil value is returned.
func DigestFile(path ...string) hash.Hash {
	src, err := os.Open(filepath.Join(path...))
	if err != nil {
		debugx.Println("failed to digest file", err)
		return nil
	}
	defer src.Close()

	return DigestIO(src)
}

// String to hash base64 encoded string
func String(s string) string {
	return FormatBase64(Digest(s))
}

// format hash to a base64 encoded string
func FormatBase64(m hash.Hash) string {
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// format hash to a hex encoded string
func FormatHex(m hash.Hash) string {
	return hex.EncodeToString(m.Sum(nil))
}
