package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestFromString(t *testing.T) {
	t.Run("should parse a canonical uuid", func(t *testing.T) {
		u, err := FromString("11223344-5566-7788-99aa-bbccddeeff00")
		require.NoError(t, err)
		require.Equal(t, []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00}, u.Bytes())
	})

	t.Run("should return an error for malformed input", func(t *testing.T) {
		_, err := FromString("not-a-uuid")
		require.Error(t, err)
	})

	t.Run("should return an error for empty input", func(t *testing.T) {
		_, err := FromString("")
		require.Error(t, err)
	})
}

func TestFromStringOrNil(t *testing.T) {
	t.Run("should parse a valid uuid", func(t *testing.T) {
		require.Equal(t, MustParse("11223344-5566-7788-99aa-bbccddeeff00"), FromStringOrNil("11223344-5566-7788-99aa-bbccddeeff00"))
	})

	t.Run("should return Nil for malformed input", func(t *testing.T) {
		require.Equal(t, Nil(), FromStringOrNil("not-a-uuid"))
	})

	t.Run("should return Nil for empty input", func(t *testing.T) {
		require.Equal(t, Nil(), FromStringOrNil(""))
	})
}
