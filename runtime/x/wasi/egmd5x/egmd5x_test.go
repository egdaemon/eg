package egmd5x_test

import (
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egmd5x"
	"github.com/stretchr/testify/require"
)

func TestDigest(t *testing.T) {
	require.Equal(t, "5d41402abc4b2a76b9719d911017c592", egmd5x.FormatHex(egmd5x.Digest("hello")))
	require.Equal(t, "5d41402abc4b2a76b9719d911017c592", egmd5x.FormatHex(egmd5x.Digest([]byte("hello"))))
	require.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", egmd5x.FormatHex(egmd5x.Digest("")))
}

func TestFormatString(t *testing.T) {
	require.Equal(t, "5d41402a-bc4b-2a76-b971-9d911017c592", egmd5x.FormatString(egmd5x.Digest("hello")))
}

func TestString(t *testing.T) {
	require.Equal(t, "5d41402a-bc4b-2a76-b971-9d911017c592", egmd5x.String("hello"))
	require.Equal(t, egmd5x.FormatString(egmd5x.Digest("hello")), egmd5x.String("hello"))
}
