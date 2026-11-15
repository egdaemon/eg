package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestBytes(t *testing.T) {
	t.Run("should return all Size bytes in order", func(t *testing.T) {
		require.Equal(t, []byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00}, MustParse("11223344-5566-7788-99aa-bbccddeeff00").Bytes())
	})

	t.Run("should return zeros for Nil", func(t *testing.T) {
		require.Equal(t, make([]byte, Size), Nil().Bytes())
	})

	t.Run("should not let callers mutate the original uuid", func(t *testing.T) {
		u := MustParse("11223344-5566-7788-99aa-bbccddeeff00")
		b := u.Bytes()
		b[0] = 0x00
		require.Equal(t, byte(0x11), u[0])
	})
}
