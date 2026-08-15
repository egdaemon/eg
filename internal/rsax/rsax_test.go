package rsax

import (
	"crypto/md5"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeterministic(t *testing.T) {
	for _, tc := range []struct {
		name     string
		seed     string
		bits     int
		expected string
	}{
		{"example 1", "helloworld", 4096, "e6dc771f6e05513c0f80fef59f643491"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pkey, err := Deterministic([]byte(tc.seed), tc.bits)
			require.NoError(t, err)
			digest := md5.Sum(pkey)
			require.Equal(t, tc.expected, hex.EncodeToString(digest[:]))
		})
	}
}
