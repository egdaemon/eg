package md5x_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/md5x"
	"github.com/stretchr/testify/require"
)

func TestFormatNHex(t *testing.T) {
	full := md5x.FormatHex(md5x.Digest("hello"))
	require.Equal(t, full[:16], md5x.FormatNHex(16, md5x.Digest("hello")))
	require.Equal(t, full, md5x.FormatNHex(1000, md5x.Digest("hello")), "longer than the digest is the whole digest")
	require.Empty(t, md5x.FormatNHex(0, md5x.Digest("hello")))
}
