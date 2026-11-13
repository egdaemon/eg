package egenv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvironOutsideWasm(t *testing.T) {
	_, err := Environ()
	require.Error(t, err)
}
