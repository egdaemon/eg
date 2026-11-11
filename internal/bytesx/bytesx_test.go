package bytesx_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/stretchr/testify/require"
)

func TestUnitMarshalTextIsLossless(t *testing.T) {
	for _, c := range []struct {
		name     string
		value    bytesx.Unit
		expected string
	}{
		{name: "zero", value: 0, expected: "0 B"},
		{name: "bytes", value: 1, expected: "1 B"},
		{name: "not a multiple of a kibibyte", value: 1234567, expected: "1234567 B"},
		{name: "kibibytes", value: bytesx.KiB, expected: "1 KiB"},
		{name: "not a multiple of a mebibyte", value: 1536 * bytesx.KiB, expected: "1536 KiB"},
		{name: "mebibyte", value: bytesx.MiB, expected: "1 MiB"},
		{name: "fractional gibibytes", value: 1536 * bytesx.MiB, expected: "1536 MiB"},
		{name: "gibibytes", value: 4 * bytesx.GiB, expected: "4 GiB"},
		{name: "a gibibyte and a byte", value: bytesx.GiB + 1, expected: "1073741825 B"},
		{name: "tebibytes", value: 2 * bytesx.TiB, expected: "2 TiB"},
		{name: "pebibytes", value: 100 * bytesx.PiB, expected: "100 PiB"},
		{name: "exbibyte", value: bytesx.EiB, expected: "1 EiB"},
		{name: "max", value: bytesx.Unit(math.MaxUint64), expected: fmt.Sprintf("%d B", uint64(math.MaxUint64))},
	} {
		t.Run(c.name, func(t *testing.T) {
			text, err := c.value.MarshalText()
			require.NoError(t, err)
			require.Equal(t, c.expected, string(text))

			// values that don't fit a float64 mantissa aren't expected to survive the parser.
			if uint64(c.value) > 1<<53 {
				return
			}

			var decoded bytesx.Unit
			require.NoError(t, decoded.UnmarshalText(text))
			require.Equal(t, c.value, decoded)
		})
	}
}

func TestUnitStringIsLossless(t *testing.T) {
	require.Equal(t, "1536 MiB", bytesx.Unit(1536*bytesx.MiB).String())
	require.Equal(t, "1234567 B", bytesx.Unit(1234567).String())

	// fmt's verbs keep the compact format.
	require.Equal(t, "2g", fmt.Sprintf("%v", bytesx.Unit(2*bytesx.GiB+1)))
}
