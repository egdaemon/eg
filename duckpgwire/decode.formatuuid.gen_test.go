package duckpgwire

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeFormatUUID(t *testing.T) {
	var b [16]byte
	for i := range b {
		b[i] = byte(i)
	}

	got := formatUUID(b)
	require.Equal(t, "00010203-0405-0607-0809-0a0b0c0d0e0f", got)
}
