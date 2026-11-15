package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestValue(t *testing.T) {
	t.Run("should be the string form", func(t *testing.T) {
		v, err := MustParse("11223344-5566-7788-99aa-bbccddeeff00").Value()
		require.NoError(t, err)
		require.Equal(t, "11223344-5566-7788-99aa-bbccddeeff00", v)
	})

	t.Run("should be the zero string form for Nil", func(t *testing.T) {
		v, err := Nil().Value()
		require.NoError(t, err)
		require.Equal(t, "00000000-0000-0000-0000-000000000000", v)
	})
}

func TestScan(t *testing.T) {
	t.Run("should reset to Nil for a nil source", func(t *testing.T) {
		u := NewV4()
		require.NoError(t, u.Scan(nil))
		require.Equal(t, Nil(), u)
	})

	t.Run("should parse a string source", func(t *testing.T) {
		var u UUID
		require.NoError(t, u.Scan("11223344-5566-7788-99aa-bbccddeeff00"))
		require.Equal(t, MustParse("11223344-5566-7788-99aa-bbccddeeff00"), u)
	})

	t.Run("should return an error for a malformed string source", func(t *testing.T) {
		var u UUID
		require.Error(t, u.Scan("not-a-uuid"))
	})

	t.Run("should accept the raw Size bytes", func(t *testing.T) {
		raw := []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00}
		var u UUID
		require.NoError(t, u.Scan(raw))
		require.Equal(t, raw, u.Bytes())
	})

	t.Run("should accept the text form as bytes", func(t *testing.T) {
		var u UUID
		require.NoError(t, u.Scan([]byte("11223344-5566-7788-99aa-bbccddeeff00")))
		require.Equal(t, MustParse("11223344-5566-7788-99aa-bbccddeeff00"), u)
	})

	t.Run("should return an error for malformed bytes", func(t *testing.T) {
		var u UUID
		require.Error(t, u.Scan([]byte("short")))
	})

	t.Run("should return an error for an unsupported source type", func(t *testing.T) {
		var u UUID
		require.Error(t, u.Scan(42))
	})

	t.Run("should round trip with Value", func(t *testing.T) {
		in := NewV7()
		v, err := in.Value()
		require.NoError(t, err)

		var out UUID
		require.NoError(t, out.Scan(v))
		require.Equal(t, in, out)
	})
}
