package lan_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/lan"
	"github.com/stretchr/testify/require"
)

func TestEncodeList(t *testing.T) {
	require.Equal(t, "[]", lan.EncodeList(nil))
	require.Equal(t, `["a","b"]`, lan.EncodeList([]string{"a", "b"}))
}
