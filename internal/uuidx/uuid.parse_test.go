package uuidx_test

import (
	"testing"

	. "github.com/egdaemon/eg/internal/uuidx"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Run("should accept upper and lower case hex", func(t *testing.T) {
		lower, err := Parse("11223344-5566-7788-99aa-bbccddeeff00")
		require.NoError(t, err)
		upper, err := Parse("11223344-5566-7788-99AA-BBCCDDEEFF00")
		require.NoError(t, err)
		require.Equal(t, lower, upper)
	})

	t.Run("should accept the compact form without dashes", func(t *testing.T) {
		compact, err := Parse("112233445566778899aabbccddeeff00")
		require.NoError(t, err)
		require.Equal(t, MustParse("11223344-5566-7788-99aa-bbccddeeff00"), compact)
	})

	t.Run("should accept braces and urn forms", func(t *testing.T) {
		braced, err := Parse("{11223344-5566-7788-99aa-bbccddeeff00}")
		require.NoError(t, err)
		urn, err := Parse("urn:uuid:11223344-5566-7788-99aa-bbccddeeff00")
		require.NoError(t, err)
		require.Equal(t, braced, urn)
	})

	t.Run("should return an error for non hex characters", func(t *testing.T) {
		_, err := Parse("zzzzzzzz-5566-7788-99aa-bbccddeeff00")
		require.Error(t, err)
	})

	t.Run("should return an error for a short uuid", func(t *testing.T) {
		_, err := Parse("11223344-5566-7788-99aa")
		require.Error(t, err)
	})
}

func TestMustParse(t *testing.T) {
	t.Run("should return the parsed uuid", func(t *testing.T) {
		require.Equal(t, "11223344-5566-7788-99aa-bbccddeeff00", MustParse("11223344-5566-7788-99aa-bbccddeeff00").String())
	})

	t.Run("should panic on malformed input", func(t *testing.T) {
		require.Panics(t, func() { MustParse("not-a-uuid") })
	})
}
