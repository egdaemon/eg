package eggit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashFormat(t *testing.T) {
	const (
		fulllength     = "211a6f2a398230a29830a31a9cbf422be74d407f"
		customlength1  = "2"
		customlength2  = "21"
		customlength6  = "211a6f"
		shortlength    = "211a6f2"
		customlength14 = "211a6f2a398230"
		customlength15 = "211a6f2a398230a"
		customlength19 = "211a6f2a398230a2983"
		customlength20 = "211a6f2a398230a29830"
	)

	ex := nhash(fulllength)
	require.Equal(t, customlength1, fmt.Sprintf("%1s", ex))
	require.Equal(t, customlength2, fmt.Sprintf("%2s", ex))

	require.Equal(t, customlength6, fmt.Sprintf("%6s", ex))
	require.Equal(t, shortlength, fmt.Sprintf("%7s", ex))

	require.Equal(t, customlength14, fmt.Sprintf("%14s", ex))
	require.Equal(t, customlength15, fmt.Sprintf("%15s", ex))

	require.Equal(t, customlength19, fmt.Sprintf("%19s", ex))
	require.Equal(t, customlength20, fmt.Sprintf("%20s", ex))
	require.Equal(t, fulllength, fmt.Sprintf("%s", ex))
}
