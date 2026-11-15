package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestFromBytes(t *testing.T) {
	t.Run("should build a uuid from exactly Size bytes", func(t *testing.T) {
		raw := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00}
		u, err := FromBytes(raw)
		require.NoError(t, err)
		require.Equal(t, raw, u.Bytes())
	})

	t.Run("should return an error and Nil for short input", func(t *testing.T) {
		u, err := FromBytes(make([]byte, Size-1))
		require.Error(t, err)
		require.Equal(t, Nil(), u)
	})

	t.Run("should return an error and Nil for long input", func(t *testing.T) {
		u, err := FromBytes(make([]byte, Size+1))
		require.Error(t, err)
		require.Equal(t, Nil(), u)
	})

	t.Run("should return an error for nil input", func(t *testing.T) {
		_, err := FromBytes(nil)
		require.Error(t, err)
	})

	t.Run("should copy the input rather than alias it", func(t *testing.T) {
		raw := make([]byte, Size)
		u, err := FromBytes(raw)
		require.NoError(t, err)
		raw[0] = 0xff
		require.Equal(t, byte(0), u[0])
	})
}

func TestFromBytesOrNil(t *testing.T) {
	t.Run("should build a uuid from exactly Size bytes", func(t *testing.T) {
		raw := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00}
		require.Equal(t, raw, FromBytesOrNil(raw).Bytes())
	})

	t.Run("should return Nil for short input", func(t *testing.T) {
		require.Equal(t, Nil(), FromBytesOrNil(make([]byte, Size-1)))
	})

	t.Run("should return Nil for long input", func(t *testing.T) {
		require.Equal(t, Nil(), FromBytesOrNil(make([]byte, Size+1)))
	})

	t.Run("should return Nil for nil input", func(t *testing.T) {
		require.Equal(t, Nil(), FromBytesOrNil(nil))
	})
}
